package core

import (
	"math/big"
	"testing"
)

func TestDivRound(t *testing.T) {
	cases := []struct {
		n, d, want int64
	}{
		{0, 10, 0},
		{14, 10, 1},
		{15, 10, 2}, // half rounds away from zero, not to even
		{25, 10, 3},
		{16, 10, 2},
		{-14, 10, -1},
		{-15, 10, -2},
		{-25, 10, -3},
		{26664, 10, 2666},   // fixture case 6: 2666.4
		{254067, 10, 25407}, // fixture case 4: 25406.7
		{9, 2, 5},
		{-9, 2, -5},
		{7, 7, 1},
	}
	for _, c := range cases {
		if got := divRound(c.n, c.d); got != c.want {
			t.Errorf("divRound(%d, %d) = %d, want %d", c.n, c.d, got, c.want)
		}
	}
}

func TestDivRoundAgainstBigRat(t *testing.T) {
	// Cross-check the integer helper against exact rational arithmetic for
	// every denominator the pricing engine uses.
	denominators := []int64{36000, 20, 10, 3600}
	numerators := []int64{0, 1, 17, 999, 26664, 26665, 95990400, 254067, -254067, -26665}
	for _, d := range denominators {
		for _, n := range numerators {
			want := ratRound(n, d)
			if got := divRound(n, d); got != want {
				t.Errorf("divRound(%d, %d) = %d, big.Rat says %d", n, d, got, want)
			}
		}
	}
}

// ratRound is the reference implementation: exact rational, half away from
// zero, no floats.
func ratRound(n, d int64) int64 {
	value := big.NewRat(n, d)
	doubled := new(big.Rat).Add(value, value)
	floor := new(big.Int).Quo(doubled.Num(), doubled.Denom()) // truncates toward zero
	half := floor.Int64()
	rounded := half / 2
	if half%2 != 0 {
		if half > 0 {
			rounded++
		} else {
			rounded--
		}
	}
	return rounded
}
