package negotiation

import (
	"io"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func zeroLogger() zerolog.Logger { return zerolog.New(io.Discard) }

func TestCounterConcedesStepByStepAndNeverPassesTheCeiling(t *testing.T) {
	m := Mandate{TargetUnitMinor: 4000, MaxUnitMinor: 4600, MaxRounds: 2, AutoCounter: true}
	cases := []struct {
		offer int64
		round int
		want  int64
		ok    bool
	}{
		{4800, 1, 4200, true}, // a third of the way to the ceiling
		{4800, 2, 4400, true}, // two thirds
		{4300, 2, 4100, true}, // below the offer: two thirds of 40→42.99, tidied to a whole unit
		{4000, 1, 0, false},   // at the target: no counter
		{4800, 3, 0, false},   // no round left
		{4001, 1, 4000, true}, // just above the target: the target itself is proposed
	}
	for _, c := range cases {
		got, ok := Counter(m, c.offer, c.round)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("offer %d round %d: got %d %v, want %d %v", c.offer, c.round, got, ok, c.want, c.ok)
		}
		if ok && (got > m.MaxUnitMinor || got >= c.offer || got < m.TargetUnitMinor) {
			t.Errorf("out of the mandate: %d", got)
		}
	}
	m.AutoCounter = false
	if _, ok := Counter(m, 4800, 1); ok {
		t.Error("countered without the buyer's permission")
	}
}

func TestGuardRefusesCountersOutsideTheMandate(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	n := Negotiation{Status: StatusNegotiating, Deadline: now.Add(time.Hour)}
	m := Mandate{TargetUnitMinor: 4000, MaxUnitMinor: 4600, MaxRounds: 2}
	p := Participant{Status: PResponded, Round: 1}
	if err := CheckCounter(n, m, p, 4800, 4200, true, now); err != nil {
		t.Fatalf("a fair counter: %v", err)
	}
	refused := map[string]error{
		"above the ceiling":   CheckCounter(n, m, p, 4800, 4700, true, now),
		"below the target":    CheckCounter(n, m, p, 4800, 3900, true, now),
		"not below the offer": CheckCounter(n, m, p, 4200, 4200, false, now),
		"no round left":       CheckCounter(n, m, Participant{Status: PResponded, Round: 3}, 4800, 4200, true, now),
		"after the deadline":  CheckCounter(n, m, p, 4800, 4200, true, now.Add(2*time.Hour)),
		"closed":              CheckCounter(Negotiation{Status: StatusAccepted, Deadline: n.Deadline}, m, p, 4800, 4200, true, now),
	}
	for name, err := range refused {
		if err == nil {
			t.Errorf("%s: allowed", name)
		}
	}
	// The buyer's own counter may go below the target.
	if err := CheckCounter(n, m, p, 4800, 3500, false, now); err != nil {
		t.Errorf("the buyer's own counter: %v", err)
	}
}

func TestGuardKeepsTheCeilingOutOfMessages(t *testing.T) {
	m := Mandate{MaxUnitMinor: 4600}
	leaks := []string{"Mon budget max est 46,00 USD", "jusqu'à $46", "46 dollars l'unité", "prix 46.00 usd", "46 $"}
	for _, text := range leaks {
		if CheckOutgoing(text, m) == nil {
			t.Errorf("not caught: %q", text)
		}
	}
	fine := []string{"Répondez avant le 46/11", "46 sacs de ciment", "146,00 USD", "46,50 USD", "Contre-proposition à 42,00 USD"}
	for _, text := range fine {
		if err := CheckOutgoing(text, m); err != nil {
			t.Errorf("refused %q: %v", text, err)
		}
	}
	// A price the buyer chose to propose may equal the ceiling.
	if err := CheckOutgoing("Contre-proposition à 46,00 USD", m, 4600); err != nil {
		t.Errorf("disclosed on purpose: %v", err)
	}
	big := Mandate{MaxUnitMinor: 4200000}
	if CheckOutgoing("plafond 42 000 FC", big) == nil || CheckOutgoing("42 000 FC", big) == nil {
		t.Error("francs with thousands not caught")
	}
}

func TestMoneyAndPhones(t *testing.T) {
	if got := Money(4250, "USD"); got != "42,50 USD" {
		t.Errorf("usd %q", got)
	}
	if got := Money(4200000, "CDF"); got != "42 000 FC" {
		t.Errorf("cdf %q", got)
	}
	for in, want := range map[string]string{"0821714747": "+243821714747", "+243 82 171 4747": "+243821714747", "243821714747": "+243821714747", "abc": "", "": ""} {
		if got := normalizePhone(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}
