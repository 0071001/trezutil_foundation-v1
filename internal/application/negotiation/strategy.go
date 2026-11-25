package negotiation

/*
 * The strategy: deterministic rules that propose what to do on the buyer's behalf. It proposes; the guard decides.
 *
 * When a supplier answers above the buyer's target and a round is left, Trezutil counters. The counter starts near
 * the target and concedes a little more each round, never above the ceiling and always below the supplier's offer:
 *
 *     counter = target + (min(ceiling, offer − 1) − target) × round ÷ (rounds + 1)
 *
 * so with 3 rounds the first counter goes a quarter of the way, the second half, the third three quarters. An offer
 * at or below the target is not countered: it is put before the buyer.
 */

// Counter returns the counter-offer for a supplier's offer at the given round, or false when none should be made.
func Counter(m Mandate, supplierUnit int64, round int) (int64, bool) {
	if !m.AutoCounter || supplierUnit <= m.TargetUnitMinor || round < 1 || round > m.MaxRounds {
		return 0, false
	}
	top := m.MaxUnitMinor
	if supplierUnit-1 < top {
		top = supplierUnit - 1
	}
	if top < m.TargetUnitMinor {
		return 0, false
	}
	price := m.TargetUnitMinor + (top-m.TargetUnitMinor)*int64(round)/int64(m.MaxRounds+1)
	price = tidy(price, m.TargetUnitMinor, top)
	if price < m.TargetUnitMinor || price > top {
		return 0, false
	}
	return price, true
}

// tidy rounds a price down to a whole unit when it is large enough to make cents look odd (45.37 → 45.00), staying
// within [low, high].
func tidy(price, low, high int64) int64 {
	if price >= 1000 {
		rounded := price - price%100
		if rounded >= low && rounded <= high {
			return rounded
		}
	}
	return price
}

// Satisfies reports whether a supplier's offer meets the buyer's target.
func Satisfies(m Mandate, supplierUnit int64) bool { return supplierUnit <= m.TargetUnitMinor }
