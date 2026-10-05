package conversation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/application/vertical"
	"github.com/trezutil/trezutil/internal/domain"
)

// What the assistant knows about the conversation so far, handed to the model
// with every message so that a short follow-up ("moins cher", "le deuxième",
// "pour 3 nuits") is read in the light of what was just shown. It contains only
// what the person already saw in this conversation: criteria, titles, prices
// and the identifiers of the numbered results.

const maxContextResults = 8

// criteriaSentence renders search criteria in plain words.
func criteriaSentence(c *domain.SearchCriteria) string {
	if c == nil || c.IsZero() {
		return ""
	}
	var parts []string
	if c.Vertical != "" {
		parts = append(parts, "vertical "+string(c.Vertical))
	}
	if c.Location != "" {
		parts = append(parts, "place "+c.Location)
	}
	if c.Nights > 0 {
		parts = append(parts, fmt.Sprintf("%d night(s)", c.Nights))
	}
	if c.PartySize > 0 {
		parts = append(parts, fmt.Sprintf("%d people", c.PartySize))
	}
	if c.StartAt != "" {
		parts = append(parts, "from "+c.StartAt)
	}
	if c.Text != "" {
		parts = append(parts, fmt.Sprintf("words %q", c.Text))
	}
	switch {
	case c.PriceMin > 0 && c.PriceMax > 0:
		parts = append(parts, fmt.Sprintf("budget %s to %s USD", trimFloat(c.PriceMin), trimFloat(c.PriceMax)))
	case c.PriceMax > 0:
		parts = append(parts, fmt.Sprintf("budget up to %s USD", trimFloat(c.PriceMax)))
	case c.PriceMin > 0:
		parts = append(parts, fmt.Sprintf("budget from %s USD", trimFloat(c.PriceMin)))
	}
	switch c.Sort {
	case "price_asc":
		parts = append(parts, "cheapest first")
	case "price_desc":
		parts = append(parts, "most expensive first")
	case "newest":
		parts = append(parts, "newest first")
	}
	if c.CategoryID != "" {
		parts = append(parts, "category_id "+c.CategoryID)
	}
	return strings.Join(parts, ", ")
}

func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// classifierContext summarises the conversation for the intent interpreter.
func classifierContext(cx *domain.ConversationContext, signedIn bool, lang string, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- Today is %s (use it for \"tonight\", \"tomorrow\", \"this weekend\", \"samedi\").\n", now.Format("Monday 2006-01-02"))
	fmt.Fprintf(&b, "- The person writes in %s.\n", map[bool]string{true: "French", false: "English"}[isFrench(lang)])
	fmt.Fprintf(&b, "- Signed in: %s.\n", map[bool]string{true: "yes", false: "no (guest: browsing only)"}[signedIn])
	if cx == nil {
		return b.String()
	}
	if s := criteriaSentence(cx.SearchCriteria); s != "" {
		fmt.Fprintf(&b, "- Last search: %s.\n", s)
	}
	if n := len(cx.ResultItems); n > 0 {
		if n == 1 && cx.ResultItems[0].RefType == "listing" {
			it := cx.ResultItems[0]
			fmt.Fprintf(&b, "- Listing in focus (the person is looking at it): %q (%s) [id %s].\n", it.Title, it.Subtitle, it.ID)
		} else {
			b.WriteString("- Results shown, numbered as the person sees them:\n")
			for i, it := range cx.ResultItems {
				if i == maxContextResults {
					fmt.Fprintf(&b, "  …and %d more.\n", n-i)
					break
				}
				fmt.Fprintf(&b, "  %d. %s — %s [%s id %s]\n", i+1, it.Title, it.Subtitle, it.RefType, it.ID)
			}
		}
	}
	if cx.CurrentOrder != nil {
		fmt.Fprintf(&b, "- Order in focus: id %s, status %s.\n", cx.CurrentOrder.ID, cx.CurrentOrder.Status)
	}
	if cx.CurrentReservation != nil {
		fmt.Fprintf(&b, "- Reservation in focus: id %s, status %s.\n", cx.CurrentReservation.ID, cx.CurrentReservation.Status)
	}
	if len(cx.MissingParameters) > 0 {
		names := make([]string, 0, len(cx.MissingParameters))
		for _, m := range cx.MissingParameters {
			names = append(names, m.Name)
		}
		fmt.Fprintf(&b, "- The assistant just asked the person for: %s. Their next message most likely answers it.\n", strings.Join(names, ", "))
	}
	if cx.HasPendingConfirmation() {
		b.WriteString("- A confirmation is pending; a plain yes or no is handled elsewhere.\n")
	}
	return b.String()
}

// refineSearch recognises a message that only adjusts the previous search ("moins
// cher", "seulement à louer", "et à Limete", "moins de 300 $") and returns the
// previous criteria with the change applied. It is the keyword-routing
// counterpart of the model reading the same follow-up with the context above.
func refineSearch(prev *domain.SearchCriteria, text string) (map[string]interface{}, bool) {
	if prev == nil || prev.IsZero() {
		return nil, false
	}
	params := keywordSearchParams(text)
	if !isRefinement(text, params) {
		return nil, false
	}
	if params["location"] == nil {
		if place, _ := extractPlace(text); place != "" {
			params["location"] = place
			if t, ok := params["text"].(string); ok {
				_, rest := extractPlace(t)
				if rest = strings.TrimSpace(withoutWords(rest, searchFillers)); rest == "" {
					delete(params, "text")
				} else {
					params["text"] = rest
				}
			}
		}
	}
	merged := map[string]interface{}{}
	set := func(k string, v interface{}) {
		switch x := v.(type) {
		case string:
			if x != "" {
				merged[k] = x
			}
		case int:
			if x > 0 {
				merged[k] = x
			}
		case float64:
			if x > 0 {
				merged[k] = x
			}
		}
	}
	set("vertical", string(prev.Vertical))
	set("location", prev.Location)
	set("nights", prev.Nights)
	set("party_size", prev.PartySize)
	set("text", prev.Text)
	set("price_min", prev.PriceMin)
	set("price_max", prev.PriceMax)
	set("sort", prev.Sort)
	set("category_id", prev.CategoryID)
	for k, v := range params {
		merged[k] = v
	}
	return merged, true
}

// enrichSearchParams completes a search the model interpreted. Models tend to
// return only what is most obvious (the vertical) and leave the rest out, so
// the details the text states plainly (a known neighbourhood, a budget, an
// ordering, a night count) are read deterministically and filled in, and a
// follow-up that only adjusts the last search inherits that search's criteria.
// Whatever the model did return is kept as it is.
func enrichSearchParams(params map[string]interface{}, text string, prev *domain.SearchCriteria) map[string]interface{} {
	if params == nil {
		params = map[string]interface{}{}
	}
	missing := func(k string) bool {
		v, ok := params[k]
		if !ok || v == nil {
			return true
		}
		switch x := v.(type) {
		case string:
			return strings.TrimSpace(x) == ""
		case float64:
			return x == 0
		case int:
			return x == 0
		}
		return false
	}

	kw := keywordSearchParams(text)
	for _, k := range []string{"price_min", "price_max", "sort", "nights", "party_size", "location"} {
		if v, ok := kw[k]; ok && missing(k) {
			params[k] = v
		}
	}
	if missing("location") {
		if place, _ := extractPlace(text); place != "" {
			params["location"] = place
		}
	}

	// A budget or an ordering within the same kind of offer adjusts the last
	// search even when other words come with it ("moins cher avec piscine").
	refines := false
	if prev != nil && !prev.IsZero() {
		sameKind := params["vertical"] == nil || params["vertical"] == string(prev.Vertical)
		adjusts := kw["price_min"] != nil || kw["price_max"] != nil || kw["sort"] != nil
		refines = isRefinement(text, kw) || (adjusts && sameKind && len(strings.Fields(text)) <= 10)
	}
	// Free words of a product search ("un téléphone Samsung") are the model's to
	// give; when it left them out and the text names no catalogue vertical, the
	// words that remain after dropping fillers and time words are the query.
	if kw["vertical"] == nil && missing("text") && (params["vertical"] == nil || params["vertical"] == "RETAIL" || params["vertical"] == "OTHER") {
		if t, _ := kw["text"].(string); t != "" {
			if cleaned := strings.TrimSpace(withoutWords(t, append(append([]string{}, searchFillers...), temporalWords...))); cleaned != "" {
				params["text"] = cleaned
			}
		}
	}

	if refines {
		carry := func(k string, v interface{}) {
			switch x := v.(type) {
			case string:
				if x == "" {
					return
				}
			case int:
				if x == 0 {
					return
				}
			case float64:
				if x == 0 {
					return
				}
			}
			if missing(k) {
				params[k] = v
			}
		}
		carry("vertical", string(prev.Vertical))
		carry("location", prev.Location)
		carry("nights", prev.Nights)
		carry("party_size", prev.PartySize)
		carry("text", prev.Text)
		carry("price_min", prev.PriceMin)
		carry("price_max", prev.PriceMax)
		carry("sort", prev.Sort)
		carry("category_id", prev.CategoryID)
	}
	return params
}

// temporalWords carry a date or a state, not a thing to search for.
var temporalWords = []string{
	"aujourd", "hui", "demain", "soir", "matin", "ce", "cette", "cet", "week", "end", "weekend", "samedi", "dimanche",
	"lundi", "mardi", "mercredi", "jeudi", "vendredi", "today", "tomorrow", "tonight", "this", "disponible", "disponibles",
	"available", "libre", "free", "maintenant", "now", "urgent", "vite", "asap",
}

var (
	// A date or time answer: 2026-10-12, 12/10, 11:00, 14h, or a day word. A bare number is not
	// one ("300" in "max 300 dollars").
	dateWordRe   = regexp.MustCompile(`(?i)(?:\d{4}-\d{1,2}-\d{1,2}|\d{1,2}[/.]\d{1,2}|\d{1,2}\s*(?:h|:)\s*\d{0,2}|aujourd|demain|lendemain|apr[èe]s|ce\s+soir|cette\s+nuit|matin|\bsoir|week|semaine|samedi|dimanche|lundi|mardi|mercredi|jeudi|vendredi|today|tomorrow|tonight|monday|tuesday|wednesday|thursday|friday|saturday|sunday|next|prochain)`)
	numberWordRe = regexp.MustCompile(`(?i)(?:\d|\b(?:un|une|deux|trois|quatre|cinq|six|sept|huit|neuf|dix|seul|seule|one|two|three|four|five|six|seven|eight|nine|ten|alone|couple|semaine|week)\b)`)
)

// answersMissingParam reports whether a reply plausibly answers the question the
// assistant just asked (for the parameter `name`). A message that is clearly a
// new request ("un téléphone Samsung max 300 dollars" after "quand cela doit-il
// se terminer ?") is not an answer and must not be forced into the pending
// action.
func answersMissingParam(name, text string) bool {
	words := len(strings.Fields(text))
	if words == 0 {
		return false
	}
	if words > 8 {
		return false
	}
	if strings.HasSuffix(name, "_id") {
		return words <= 5
	}
	switch name {
	case "start_at", "end_at":
		return words <= 6 && dateWordRe.MatchString(text)
	case "nights", "party_size", "quantity":
		if v, _ := vertical.DefaultCatalog().DetectVertical(text); v != "" {
			return false // "plutôt un restaurant" changes the request
		}
		return words <= 4 && numberWordRe.MatchString(text)
	}
	// Free-text answers (a place, a kind of offer): short, and not a request of
	// its own with a search, booking or buying verb.
	if words > 2 && resolveActionTypeKeyword(text) != "" {
		return false
	}
	return true
}
