package searching

import (
	"net/url"
	"testing"
)

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

func TestBingSearchURL(t *testing.T) {
	u0, err := url.Parse(bingSearchURL("test", 0))
	if err != nil {
		t.Fatal(err)
	}
	q0 := u0.Query()
	if q0.Get("q") != "test" || q0.Get("first") != "1" || q0.Get("count") != "10" {
		t.Fatalf("page0 query: %v", q0)
	}
	if q0.Get("FORM") != "" {
		t.Fatalf("page0 should omit FORM, got %q", q0.Get("FORM"))
	}

	u1, err := url.Parse(bingSearchURL("test", 1))
	if err != nil {
		t.Fatal(err)
	}
	q1 := u1.Query()
	if q1.Get("first") != "11" || q1.Get("count") != "10" {
		t.Fatalf("page1 query: %v", q1)
	}
}
