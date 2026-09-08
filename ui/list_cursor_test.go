package ui

import (
	"reflect"
	"testing"
)

func TestWrappedLineCount(t *testing.T) {
	tests := []struct {
		cellLen, width, want int
	}{
		{0, 10, 1},
		{9, 10, 1},
		{10, 10, 2}, // exact multiple → trailing empty visual row (gocui)
		{11, 10, 2},
		{20, 10, 3},
		{5, 0, 6}, // width clamped to 1
	}
	for _, tc := range tests {
		if got := wrappedLineCount(tc.cellLen, tc.width); got != tc.want {
			t.Errorf("wrappedLineCount(%d, %d) = %d, want %d", tc.cellLen, tc.width, got, tc.want)
		}
	}
}

func TestVisualLineStarts(t *testing.T) {
	// width 10: "short" = 1 row; 20-char line = 3 visual rows (gocui exact-multiple quirk)
	lines := []string{
		"short",            // 5 → 1
		"abcdefghij",       // 10 → 2
		"abcdefghijABCDEF", // 16 → 2
	}
	got := visualLineStarts(lines, 10)
	want := []int{0, 1, 3, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visualLineStarts = %v, want %v", got, want)
	}
	// Buffer index 2 starts at visual row 3
	if got[2] != 3 {
		t.Fatalf("domain index 2 visual start = %d, want 3", got[2])
	}
}
