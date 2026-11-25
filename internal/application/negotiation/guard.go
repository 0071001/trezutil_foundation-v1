package negotiation

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

/*
 * The mandate guard: the final authority over what Trezutil does on the buyer's behalf. Pure code, no model.
 *
 *   - a counter-offer made for the buyer (by the strategy) lies between the target and the ceiling, below the
 *     supplier's current offer, within the rounds and before the deadline;
 *   - a counter-offer the buyer types themselves is their own decision: it only has to be below the supplier's offer,
 *     within the rounds and before the deadline;
 *   - the ceiling never appears in a message to a supplier, unless it is the very price the buyer chose to propose.
 */

// GuardError says why the guard refused.
type GuardError struct{ Reason string }

func (e *GuardError) Error() string { return "mandate: " + e.Reason }

func refuse(format string, args ...interface{}) error {
	return &GuardError{Reason: fmt.Sprintf(format, args...)}
}

// CheckCounter decides whether a counter-offer may be sent.
func CheckCounter(n Negotiation, m Mandate, p Participant, supplierUnit, counter int64, automatic bool, now time.Time) error {
	if !n.Status.Open() {
		return refuse("the negotiation is closed")
	}
	if !now.Before(n.Deadline) {
		return refuse("the deadline has passed")
	}
	if !p.Status.Active() {
		return refuse("this supplier can no longer answer")
	}
	if p.Round > m.MaxRounds {
		return refuse("no round left (%d allowed)", m.MaxRounds)
	}
	if counter <= 0 {
		return refuse("a counter-offer must be a positive price")
	}
	if counter >= supplierUnit {
		return refuse("a counter-offer must be below the supplier's offer")
	}
	if automatic {
		if counter < m.TargetUnitMinor {
			return refuse("below the buyer's target")
		}
		if counter > m.MaxUnitMinor {
			return refuse("above the buyer's ceiling")
		}
	}
	return nil
}

// CheckOutgoing refuses a message to a supplier that discloses the buyer's ceiling. A price the message deliberately
// proposes (disclosed) may equal it: then the buyer chose to say it.
func CheckOutgoing(text string, m Mandate, disclosed ...int64) error {
	for _, d := range disclosed {
		if d == m.MaxUnitMinor {
			return nil
		}
	}
	lower := strings.ToLower(text)
	for _, form := range amountForms(m.MaxUnitMinor) {
		if containsAmount(lower, form) {
			return refuse("the message would disclose the buyer's ceiling")
		}
	}
	return nil
}

// amountForms are the ways an amount in minor units may be written: 4500 for 45.00, "45", "45.00", "45,00".
func amountForms(minor int64) []string {
	major := minor / 100
	cents := minor % 100
	forms := []string{}
	if cents == 0 {
		forms = append(forms, strconv.FormatInt(major, 10), groupThousands(major, " "), groupThousands(major, " "), groupThousands(major, " "), groupThousands(major, ","), groupThousands(major, "."))
	}
	forms = append(forms, fmt.Sprintf("%d.%02d", major, cents), fmt.Sprintf("%d,%02d", major, cents))
	seen := map[string]bool{}
	out := forms[:0]
	for _, f := range forms {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

func groupThousands(n int64, sep string) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteString(sep)
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// currencyWords are what makes a number a price in a message: "45 USD", "45 $", "$45", "42 000 FC", "45 dollars".
var currencyWords = []string{"usd", "us$", "$us", "$", "cdf", "fc", "franc", "dollar", "€", "eur"}

// containsAmount finds the amount written as a price: a whole number (not part of a longer one: 45 is not in 145 or
// 45.50) next to a currency. A date or a quantity with the same digits is not a price.
func containsAmount(text, amount string) bool {
	for i := 0; ; {
		j := strings.Index(text[i:], amount)
		if j < 0 {
			return false
		}
		start, end := i+j, i+j+len(amount)
		before := start == 0 || !isAmountChar(text[start-1])
		after := end == len(text) || !isAmountChar(text[end]) || ((text[end] == '.' || text[end] == ',') && (end+1 == len(text) || !isDigit(text[end+1])))
		if before && after && (currencyAfter(text[end:]) || currencyBefore(text[:start])) {
			return true
		}
		i = start + 1
	}
}

func currencyAfter(rest string) bool {
	rest = strings.TrimLeft(rest, "   ")
	for _, w := range currencyWords {
		if strings.HasPrefix(rest, w) {
			return true
		}
	}
	return false
}

func currencyBefore(head string) bool {
	head = strings.TrimRight(head, "   ")
	for _, w := range currencyWords {
		if strings.HasSuffix(head, w) {
			return true
		}
	}
	return false
}

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isAmountChar(c byte) bool { return isDigit(c) || c == '.' || c == ',' }
