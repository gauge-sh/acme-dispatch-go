package core

import "testing"

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
