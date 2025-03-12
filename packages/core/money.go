package core

// divRound divides n by d (d > 0) rounding half away from zero. All pricing
// stays in int64 cents; the fractional inputs (hours, markups, surcharges)
// are expressed as exact ratios so no float ever touches money.
func divRound(n, d int64) int64 {
	q, r := n/d, n%d
	if r < 0 {
		r = -r
	}
	if 2*r >= d {
		if n < 0 {
			return q - 1
		}
		return q + 1
	}
	return q
}
