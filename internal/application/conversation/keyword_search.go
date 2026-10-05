package conversation

import (
	"regexp"
	"strconv"
	"strings"
)

// Details the keyword router can read from a request without a model: a known
// neighbourhood, a number of nights, a party size, a budget and an ordering, in
// French and English. The model understands far more; this keeps the basics
// working when it is unavailable.

// knownPlaces are the neighbourhoods Trezutil lists today (Kinshasa).
var knownPlaces = []string{
	"Gombe", "Limete", "Ngaliema", "Kintambo", "Bandalungwa", "Masina", "Kasa-Vubu", "Mont-Ngafula",
	"Binza", "Kimwenza", "Ma Campagne", "Kalamu", "Lingwala", "Ngiri-Ngiri", "Lemba", "Matete",
}

const (
	num = `(\d{1,9}(?:[.,]\d{1,2})?)`
	// An amount is "$600" or "600 $" / "600 dollars" / "600 usd": two capture groups, one of them set.
	amt = `(?:\$\s*` + num + `|` + num + `\s*(?:\$us|\$|usd|us\s?\$|dollars?))`
)

var (
	nightsRe = regexp.MustCompile(`(?i)\b(\d{1,2})\s*(?:nuits?|nights?)\b`)
	guestsRe = regexp.MustCompile(`(?i)\b(\d{1,2})\s*(?:personnes?|voyageurs?|people|persons?|guests?|adultes?|adults?)\b`)

	// "under $600", "moins de 600 $", "max 50 dollars", "budget 300 $"
	priceMaxRe = regexp.MustCompile(`(?i)(?:moins\s+de|moins\s+que|under|below|max(?:imum)?|jusqu['’]à|up\s+to|pas\s+plus\s+de|budget(?:\s+de|\s+of)?|inférieur(?:e)?\s+à|no\s+more\s+than|less\s+than)\s*(?:de\s+)?` + amt)
	// "1300 dollars maximum", "300 $ max", "50 usd tout au plus": the amount comes first
	priceMaxAfterRe = regexp.MustCompile(`(?i)` + amt + `\s*(?:max(?:imum)?|maxi|au\s+maximum|tout\s+au\s+plus|at\s+most|or\s+less|ou\s+moins)\b`)
	// "over $100", "plus de 100 $", "à partir de 50 dollars", "at least $20"
	priceMinRe = regexp.MustCompile(`(?i)(?:plus\s+de|à\s+partir\s+de|from|above|over|au\s+moins|at\s+least|minimum|supérieur(?:e)?\s+à|more\s+than)\s*` + amt)
	// "around $500", "autour de 500 $", "environ 500 dollars"
	priceAroundRe = regexp.MustCompile(`(?i)(?:autour\s+de|environ|around|about|approximately|vers)\s*` + amt)
	// "between 100 and 300 $", "entre 100 et 300 dollars"
	priceBetweenRe = regexp.MustCompile(`(?i)(?:entre|between)\s*\$?\s*` + num + `\s*(?:\$|usd|dollars?)?\s*(?:et|and|-|à)\s*` + amt)

	cheapestRe  = regexp.MustCompile(`(?i)\b(?:moins\s+chers?|pas\s+chers?|bon\s+march[ée]|prix\s+bas|petits?\s+prix|cheapest|cheaper|cheap|lowest\s+price|low\s+price|affordable|abordables?|[ée]conomiques?)\b`)
	priciestRe  = regexp.MustCompile(`(?i)\b(?:plus\s+chers?|haut\s+de\s+gamme|luxe|luxury|most\s+expensive|premium|high[- ]end|prix\s+[ée]lev[ée]s?)\b`)
	newestRe    = regexp.MustCompile(`(?i)\b(?:plus\s+r[ée]cents?|nouveaut[ée]s?|nouveaux?|nouvelles?|newest|latest|just\s+listed|recent)\b`)
	refineCueRe = regexp.MustCompile(`(?i)\b(?:seulement|uniquement|only|plut[ôo]t|instead|au\s+lieu|en\s+plus|aussi|also|et\s+à|et\s+dans|même\s+chose|same\s+(?:but|thing)|autres?|others?|more|encore|show\s+more)\b`)
)

// amountAt returns the number captured by an amt group pair starting at group g.
func amountAt(text string, m []int, g int) (float64, bool) {
	for _, i := range []int{g, g + 1} {
		if 2*i+1 < len(m) && m[2*i] >= 0 {
			return parseMoney(text[m[2*i]:m[2*i+1]])
		}
	}
	return 0, false
}

func parseMoney(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", "."), 64)
	return f, err == nil && f >= 0
}

// extractPlace returns the known place named in the text and the text without it.
func extractPlace(text string) (string, string) {
	lower := strings.ToLower(text)
	for _, place := range knownPlaces {
		i := strings.Index(lower, strings.ToLower(place))
		if i < 0 {
			continue
		}
		rest := strings.TrimSpace(text[:i] + " " + text[i+len(place):])
		return place, rest
	}
	return "", text
}

// extractCount reads "2 nuits" / "4 personnes" style counts and returns the
// text without the matched words.
func extractCount(re *regexp.Regexp, text string) (int, string) {
	m := re.FindStringSubmatchIndex(text)
	if m == nil {
		return 0, text
	}
	n, err := strconv.Atoi(text[m[2]:m[3]])
	if err != nil || n <= 0 {
		return 0, text
	}
	return n, strings.TrimSpace(text[:m[0]] + " " + text[m[1]:])
}

// extractBudget reads a price ceiling, floor or range from the text and returns
// the text without the matched words. "around 500" becomes 400 to 600.
func extractBudget(text string, params map[string]interface{}) string {
	if m := priceBetweenRe.FindStringSubmatchIndex(text); m != nil {
		lo, ok1 := parseMoney(text[m[2]:m[3]])
		hi, ok2 := amountAt(text, m, 2)
		if ok1 && ok2 {
			if lo > hi {
				lo, hi = hi, lo
			}
			params["price_min"], params["price_max"] = lo, hi
			return strings.TrimSpace(text[:m[0]] + " " + text[m[1]:])
		}
	}
	if m := priceAroundRe.FindStringSubmatchIndex(text); m != nil {
		if v, ok := amountAt(text, m, 1); ok && v > 0 {
			params["price_min"], params["price_max"] = v*0.8, v*1.2
			return strings.TrimSpace(text[:m[0]] + " " + text[m[1]:])
		}
	}
	if m := priceMaxRe.FindStringSubmatchIndex(text); m != nil {
		if v, ok := amountAt(text, m, 1); ok {
			params["price_max"] = v
			text = strings.TrimSpace(text[:m[0]] + " " + text[m[1]:])
		}
	}
	if _, set := params["price_max"]; !set {
		if m := priceMaxAfterRe.FindStringSubmatchIndex(text); m != nil {
			if v, ok := amountAt(text, m, 1); ok {
				params["price_max"] = v
				text = strings.TrimSpace(text[:m[0]] + " " + text[m[1]:])
			}
		}
	}
	if m := priceMinRe.FindStringSubmatchIndex(text); m != nil {
		if v, ok := amountAt(text, m, 1); ok {
			params["price_min"] = v
			text = strings.TrimSpace(text[:m[0]] + " " + text[m[1]:])
		}
	}
	return text
}

// extractSort reads an ordering wish ("les moins chers", "cheapest first").
func extractSort(text string, params map[string]interface{}) string {
	for _, c := range []struct {
		re   *regexp.Regexp
		sort string
	}{{cheapestRe, "price_asc"}, {priciestRe, "price_desc"}, {newestRe, "newest"}} {
		if loc := c.re.FindStringIndex(text); loc != nil {
			params["sort"] = c.sort
			return strings.TrimSpace(text[:loc[0]] + " " + text[loc[1]:])
		}
	}
	return text
}

// isRefinement reports whether a message only adjusts the previous search
// ("moins cher", "seulement à louer", "et à Limete", a bare budget) rather than
// starting a new one.
func isRefinement(text string, params map[string]interface{}) bool {
	if params["vertical"] != nil {
		return false
	}
	if params["price_min"] != nil || params["price_max"] != nil || params["sort"] != nil {
		// "moins cher", "moins de 300 $": only a budget or an ordering. Words left
		// over ("un téléphone Samsung max 300 $") make it a new search.
		return params["text"] == nil && len(strings.Fields(text)) <= 10
	}
	if refineCueRe.MatchString(text) && len(strings.Fields(text)) <= 8 {
		return true
	}
	// A bare place: "Limete", "à Limete".
	if place, rest := extractPlace(text); place != "" {
		rest = withoutWords(rest, searchFillers)
		return strings.TrimSpace(rest) == ""
	}
	return false
}
