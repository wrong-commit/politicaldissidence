package searching

import "testing"

func TestBingFirst(t *testing.T) {
	cases := []struct {
		page int
		want int
	}{
		{0, 1},
		{1, 11},
		{2, 21},
		{-1, 1},
	}
	for _, tc := range cases {
		if got := BingFirst(tc.page); got != tc.want {
			t.Fatalf("BingFirst(%d)=%d want %d", tc.page, got, tc.want)
		}
	}
}
