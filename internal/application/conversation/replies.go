package conversation

import (
	"fmt"
	"hash/fnv"
	"strings"
	"unicode"

	"github.com/trezutil/trezutil/internal/application/vertical"
	"github.com/trezutil/trezutil/internal/domain"
)

// Reply wording. The assistant only interprets; these helpers phrase what the
// runtime already did, in a warm and varied voice, in the language the person
// writes in (French and English). Variation is deterministic (hash of what was
// asked), so the same request always gets the same reply, while different
// requests do not all sound alike.

// DefaultLanguage answers people who did not say which language they use (a
// WhatsApp message carries no locale). The service sets it from the
// DEFAULT_LANGUAGE environment variable; it is French in production.
var DefaultLanguage = "en"

// normalizeLanguage maps a locale ("fr-CD", "en_US", "FR") to "fr" or "en".
// It returns "" for an empty or unsupported locale.
func normalizeLanguage(locale string) string {
	l := strings.ToLower(strings.TrimSpace(locale))
	switch {
	case strings.HasPrefix(l, "fr"):
		return "fr"
	case strings.HasPrefix(l, "en"):
		return "en"
	}
	return ""
}

// resolveLanguage picks the reply language for one message: the locale sent
// with it, else the language already used in the conversation, else the default.
func resolveLanguage(locale, current string) string {
	if l := normalizeLanguage(locale); l != "" {
		return l
	}
	if l := normalizeLanguage(current); l != "" {
		return l
	}
	if l := normalizeLanguage(DefaultLanguage); l != "" {
		return l
	}
	return "en"
}

func isFrench(lang string) bool { return normalizeLanguage(lang) == "fr" }

// tr returns the French text for French conversations and the English otherwise.
func tr(lang, en, fr string) string {
	if isFrench(lang) {
		return fr
	}
	return en
}

// pick returns one of the options, chosen by a stable hash of seed.
func pick(seed string, options ...string) string {
	if len(options) == 0 {
		return ""
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	return options[int(h.Sum32()%uint32(len(options)))]
}

func lowerFirst(s string) string {
	for i, r := range s {
		return string(unicode.ToLower(r)) + s[i+len(string(r)):]
	}
	return s
}

// verticalNounFR is how each vertical reads inside a French sentence.
var verticalNounFR = map[domain.Vertical]string{
	domain.VerticalRetail:        "produits",
	domain.VerticalServices:      "services",
	domain.VerticalHotel:         "hôtels et hébergements",
	domain.VerticalRestaurant:    "restaurants",
	domain.VerticalRealEstate:    "biens immobiliers",
	domain.VerticalVehicleRental: "locations de véhicules",
	domain.VerticalTransport:     "transports",
	domain.VerticalEvents:        "événements",
	domain.VerticalTravel:        "voyages",
	domain.VerticalOther:         "annonces",
}

func countWord(lang string, n int, en1, enN, fr1, frN string) string {
	if isFrench(lang) {
		if n == 0 || n == 1 {
			return fmt.Sprintf("%d %s", n, fr1)
		}
		return fmt.Sprintf("%d %s", n, frN)
	}
	if n == 1 {
		return fmt.Sprintf("%d %s", n, en1)
	}
	return fmt.Sprintf("%d %s", n, enN)
}

// searchPhrase describes what was searched for in plain words, e.g.
// "hotels & accommodation in Gombe for 2 nights, 2 guests". Empty when the
// search carried no structured criteria.
func searchPhrase(lang string, raw interface{}) string {
	c, ok := raw.(map[string]interface{})
	if !ok || len(c) == 0 {
		return ""
	}
	fr := isFrench(lang)
	phrase := ""
	if v, ok := c["vertical"].(string); ok {
		if def, found := vertical.DefaultCatalog().Get(domain.Vertical(v)); found {
			if fr {
				phrase = verticalNounFR[domain.Vertical(v)]
			} else {
				phrase = lowerFirst(def.Name)
			}
		}
	}
	if loc, ok := c["location"].(string); ok && loc != "" {
		if phrase == "" {
			phrase = tr(lang, "places", "lieux")
		}
		phrase += tr(lang, " in ", " à ") + loc
	}
	var extra []string
	if n := toInt(c["nights"]); n > 0 {
		extra = append(extra, countWord(lang, n, "night", "nights", "nuit", "nuits"))
	}
	if n := toInt(c["party_size"]); n > 0 {
		extra = append(extra, countWord(lang, n, "guest", "guests", "voyageur", "voyageurs"))
	}
	if len(extra) > 0 {
		if phrase == "" {
			phrase = tr(lang, "your stay", "votre séjour")
		}
		phrase += tr(lang, " for ", " pour ") + strings.Join(extra, ", ")
	}
	// Budget and ordering: "— under $600, cheapest first".
	var tail []string
	lo, hi := toFloat(c["price_min"]), toFloat(c["price_max"])
	switch {
	case lo > 0 && hi > 0:
		tail = append(tail, tr(lang, fmt.Sprintf("between $%s and $%s", trimFloat(lo), trimFloat(hi)), fmt.Sprintf("entre %s et %s $", trimFloat(lo), trimFloat(hi))))
	case hi > 0:
		tail = append(tail, tr(lang, "under $"+trimFloat(hi), "à moins de "+trimFloat(hi)+" $"))
	case lo > 0:
		tail = append(tail, tr(lang, "from $"+trimFloat(lo), "à partir de "+trimFloat(lo)+" $"))
	}
	switch c["sort"] {
	case "price_asc":
		tail = append(tail, tr(lang, "cheapest first", "du moins cher au plus cher"))
	case "price_desc":
		tail = append(tail, tr(lang, "most expensive first", "du plus cher au moins cher"))
	case "newest":
		tail = append(tail, tr(lang, "newest first", "les plus récents d’abord"))
	}
	if len(tail) > 0 {
		if phrase == "" {
			phrase = tr(lang, "your search", "votre recherche")
		}
		phrase += " — " + strings.Join(tail, ", ")
	}
	return phrase
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

func resultTitles(items []domain.ResultItemRef) string {
	var b strings.Builder
	for _, it := range items {
		b.WriteString(it.Title)
		b.WriteByte('|')
	}
	return b.String()
}

// searchText phrases a search result.
func searchText(lang string, data interface{}, items []domain.ResultItemRef) string {
	seed := resultTitles(items)
	if len(items) == 0 {
		if isFrench(lang) {
			return pick(seed+"empty",
				"Je n’ai rien trouvé qui corresponde — pour l’instant. Essayez d’autres mots, un quartier comme Gombe, ou une catégorie : hôtels, restaurants, services.",
				"Rien ne correspond pour le moment. Pouvez-vous essayer d’autres mots, ou me donner un quartier ? Je peux aussi chercher par catégorie — hôtels, restauration, services, logements.",
			)
		}
		return pick(seed+"empty",
			"I couldn't find a match for that — yet. Try different words, a neighbourhood like Gombe, or a category such as hotels, restaurants or services.",
			"Nothing matches that just now. Could you try other words, or tell me a neighbourhood? I can also look by category — hotels, food, services, homes.",
		)
	}
	var b strings.Builder
	phrase := ""
	if m, ok := data.(map[string]interface{}); ok {
		criteria := m["criteria"]
		if _, relaxed := m["relaxed"]; relaxed {
			if c, ok := criteria.(map[string]interface{}); ok && c["location"] != nil {
				fmt.Fprintf(&b, tr(lang,
					"I didn't find anything listed in %v yet, so here are the closest matches.\n",
					"Je n’ai rien trouvé à %v pour l’instant, voici donc les correspondances les plus proches.\n"), c["location"])
				rest := make(map[string]interface{}, len(c))
				for k, v := range c {
					if k != "location" {
						rest[k] = v
					}
				}
				criteria = rest
			}
		}
		phrase = searchPhrase(lang, criteria)
	}
	n := len(items)
	if isFrench(lang) {
		switch {
		case n == 1 && phrase != "":
			fmt.Fprintf(&b, "J’ai trouvé une correspondance pour %s :", phrase)
		case n == 1:
			b.WriteString(pick(seed, "J’ai trouvé une option pour vous :", "Il y en a une qui correspond :"))
		case phrase != "":
			b.WriteString(pick(seed,
				fmt.Sprintf("Bonne nouvelle — j’ai trouvé %d options pour %s. Voici les meilleures :", n, phrase),
				fmt.Sprintf("Voici %d options pour %s :", n, phrase),
				fmt.Sprintf("J’ai trouvé %d options pour %s. Jetez-y un œil :", n, phrase),
			))
		default:
			b.WriteString(pick(seed,
				fmt.Sprintf("Bonne nouvelle — j’ai trouvé %d options qui conviennent. Voici les meilleures :", n),
				fmt.Sprintf("Voici %d options qui correspondent :", n),
				fmt.Sprintf("J’ai trouvé %d options pour vous :", n),
			))
		}
	} else {
		switch {
		case n == 1 && phrase != "":
			fmt.Fprintf(&b, "I found one match for %s:", phrase)
		case n == 1:
			b.WriteString(pick(seed, "I found one match for you:", "There's one that fits:"))
		case phrase != "":
			b.WriteString(pick(seed,
				fmt.Sprintf("Good news — I found %d options for %s. Here are the top picks:", n, phrase),
				fmt.Sprintf("Here are %d options for %s:", n, phrase),
				fmt.Sprintf("I found %d options for %s. Take a look:", n, phrase),
			))
		default:
			b.WriteString(pick(seed,
				fmt.Sprintf("Good news — I found %d options that fit. Here are the top picks:", n),
				fmt.Sprintf("Here are %d options that match:", n),
				fmt.Sprintf("I found %d options for you:", n),
			))
		}
	}
	for i, it := range items {
		if i == 5 {
			b.WriteString(tr(lang, "\n…and a few more.", "\n…et quelques autres."))
			break
		}
		fmt.Fprintf(&b, "\n%d. %s", i+1, it.Title)
		if it.Subtitle != "" {
			fmt.Fprintf(&b, " — %s", it.Subtitle)
		}
	}
	if isFrench(lang) {
		b.WriteString("\n" + pick(seed+"close",
			"Dites-moi le numéro qui vous plaît et je vous donne les détails — ou dites « les moins chers d’abord » pour affiner.",
			"Lequel vous attire ? Répondez avec son numéro et je vous en dis plus.",
			"Répondez avec un numéro pour voir une fiche en détail, ou dites-moi quoi changer et je relance la recherche.",
		))
	} else {
		b.WriteString("\n" + pick(seed+"close",
			"Tell me the number you like and I'll give you the details — or say “cheapest first” to narrow it down.",
			"Which one catches your eye? Reply with its number and I'll tell you more.",
			"Reply with a number to see one in detail, or tell me what to change and I'll search again.",
		))
	}
	return b.String()
}

// listingText phrases a single listing.
func listingText(lang string, l *domain.Listing) string {
	text := l.Title
	if price := formatPrice(l.PriceAmount, l.PriceCurrency); price != "" {
		text += " — " + price
	}
	if l.Description != nil && *l.Description != "" {
		text += "\n" + *l.Description
	}
	switch l.ListingType {
	case domain.ListingTypePhysicalProduct:
		return text + "\n" + tr(lang,
			"Want it? Just say “buy it” and I'll set up the order with you.",
			"Il vous plaît ? Dites simplement « achète-le » et je prépare la commande avec vous.")
	case domain.ListingTypeService, domain.ListingTypeFood:
		return text + "\n" + tr(lang,
			"Shall I book it for you? Say “book it” — or “buy it” to order.",
			"Je vous le réserve ? Dites « réserve-le » — ou « achète-le » pour commander.")
	}
	return text + "\n" + tr(lang,
		"Would you like to book it or order it? Just say “book it” or “buy it”.",
		"Souhaitez-vous le réserver ou le commander ? Dites simplement « réserve-le » ou « achète-le ».")
}

// suggestionsFor returns up to four short follow-ups the user is likely to want
// after this reply. They are plain messages the user could type; the web app
// shows them as quick replies.
func suggestionsFor(lang string, actionType domain.CommerceActionType, data interface{}) []string {
	fr := isFrench(lang)
	pickLang := func(en, f []string) []string {
		if fr {
			return f
		}
		return en
	}
	switch actionType {
	case domain.ActionSearchListings:
		items := extractResultItems(data)
		if len(items) == 0 {
			return pickLang(
				[]string{"Hotels in Gombe", "Restaurants nearby", "An electrician available today", "Apartments to rent"},
				[]string{"Des hôtels à Gombe", "Des restaurants à proximité", "Un électricien disponible aujourd’hui", "Des appartements à louer"})
		}
		out := []string{tr(lang, "Tell me about number 1", "Parle-moi du numéro 1")}
		if len(items) > 1 {
			out = append(out, tr(lang, "Show the cheapest first", "Montre les moins chers d’abord"))
		}
		if m, ok := data.(map[string]interface{}); ok {
			if c, ok := m["criteria"].(map[string]interface{}); ok {
				switch c["vertical"] {
				case string(domain.VerticalHotel):
					out = append(out, tr(lang, "Book the first one for 2 nights", "Réserve le premier pour 2 nuits"), tr(lang, "Only with breakfast", "Seulement avec petit-déjeuner"))
				case string(domain.VerticalRestaurant):
					out = append(out, tr(lang, "Book a table for 4", "Réserve une table pour 4"), tr(lang, "Something cheaper", "Quelque chose de moins cher"))
				case string(domain.VerticalRealEstate):
					out = append(out, tr(lang, "Only to rent", "Seulement à louer"), tr(lang, "Only for sale", "Seulement à vendre"))
				default:
					out = append(out, tr(lang, "Something cheaper", "Quelque chose de moins cher"), tr(lang, "Which are available today?", "Lesquels sont disponibles aujourd’hui ?"))
				}
				return out
			}
		}
		return append(out, tr(lang, "Something cheaper", "Quelque chose de moins cher"), tr(lang, "Which are available today?", "Lesquels sont disponibles aujourd’hui ?"))
	case domain.ActionGetListing:
		back := tr(lang, "Back to the results", "Retour aux résultats")
		similar := tr(lang, "Show me something similar", "Montre-moi quelque chose de similaire")
		if l, ok := data.(*domain.Listing); ok && l != nil {
			switch l.ListingType {
			case domain.ListingTypePhysicalProduct:
				return []string{tr(lang, "Buy it", "Achète-le"), similar, back}
			case domain.ListingTypeService, domain.ListingTypeFood:
				return []string{tr(lang, "Book it", "Réserve-le"), similar, back}
			}
		}
		return []string{tr(lang, "Book it", "Réserve-le"), tr(lang, "Buy it", "Achète-le"), back}
	case domain.ActionCreateOrder:
		return pickLang([]string{"Show my orders", "Keep browsing"}, []string{"Montre mes commandes", "Continuer à explorer"})
	case domain.ActionCreateReservation:
		return pickLang([]string{"Show my reservations", "Keep browsing"}, []string{"Montre mes réservations", "Continuer à explorer"})
	case domain.ActionCheckAvailability:
		if m, ok := data.(map[string]interface{}); ok {
			if avail, _ := m["available"].(bool); avail {
				return pickLang([]string{"Book it", "Show me something similar"}, []string{"Réserve-le", "Montre-moi quelque chose de similaire"})
			}
		}
		return pickLang([]string{"Try other dates", "Show me something similar"}, []string{"Essayer d’autres dates", "Montre-moi quelque chose de similaire"})
	case domain.ActionCancelReservation, domain.ActionCancelOrder:
		return pickLang([]string{"Keep browsing", "Show my reservations"}, []string{"Continuer à explorer", "Montre mes réservations"})
	}
	return nil
}

// paramSuggestions offers tappable answers for a question the assistant asked.
var paramSuggestions = map[string][]string{
	"text":       {"A laptop for work under $600", "A hotel in Gombe this weekend", "An electrician available today", "A two-bedroom apartment"},
	"vertical":   {"A hotel", "A restaurant", "An apartment", "A service"},
	"location":   {"Gombe", "Limete", "Ngaliema", "Kintambo"},
	"nights":     {"1 night", "2 nights", "3 nights", "A week"},
	"party_size": {"1 person", "2 people", "4 people", "6 people"},
	"quantity":   {"1", "2", "3"},
}

var paramSuggestionsFR = map[string][]string{
	"text":       {"Un ordinateur portable pour le travail à moins de 600 $", "Un hôtel à Gombe ce week-end", "Un électricien disponible aujourd’hui", "Un appartement de deux chambres"},
	"vertical":   {"Un hôtel", "Un restaurant", "Un appartement", "Un service"},
	"location":   {"Gombe", "Limete", "Ngaliema", "Kintambo"},
	"nights":     {"1 nuit", "2 nuits", "3 nuits", "Une semaine"},
	"party_size": {"1 personne", "2 personnes", "4 personnes", "6 personnes"},
	"quantity":   {"1", "2", "3"},
}

func clarificationSuggestions(lang, param string) []string {
	if isFrench(lang) {
		return paramSuggestionsFR[param]
	}
	return paramSuggestions[param]
}

// welcomeSuggestions are offered when the assistant has nothing specific to follow up.
func welcomeSuggestions(lang string) []string {
	if isFrench(lang) {
		return []string{"Un hôtel à Gombe ce week-end", "Un électricien disponible aujourd’hui", "Un ordinateur portable à moins de 600 $", "Un appartement de deux chambres"}
	}
	return []string{"A hotel in Gombe this weekend", "An electrician available today", "A laptop under $600", "A two-bedroom apartment"}
}

// friendlyParamPromptsFR are the questions the assistant asks for a missing detail.
var friendlyParamPromptsFR = map[string]string{
	"text":           "Bien sûr — que cherchez-vous ? Vous pouvez dire par exemple « un hôtel à Gombe » ou « un iPhone 13 ».",
	"listing_id":     "Lequel voulez-vous dire ? Répondez avec son numéro dans la liste (par exemple « 2 » ou « le deuxième »).",
	"start_at":       "Quand voulez-vous commencer ? (par exemple 2026-10-01 14:00)",
	"end_at":         "Et quand cela doit-il se terminer ? (par exemple 2026-10-03 11:00)",
	"quantity":       "Combien en voulez-vous ?",
	"vertical":       "Quel type de lieu cherchez-vous — un hôtel, un restaurant, un appartement ?",
	"location":       "Dans quel quartier dois-je chercher ? (par exemple Gombe)",
	"nights":         "Pour combien de nuits ?",
	"party_size":     "Pour combien de personnes ?",
	"order_id":       "De quelle commande parlez-vous ? Merci d’indiquer sa référence.",
	"reservation_id": "De quelle réservation parlez-vous ? Merci d’indiquer sa référence.",
}
