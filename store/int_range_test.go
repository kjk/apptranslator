package store

import (
	"strings"
	"testing"
)

func rangeArrToStr(arr []IntRange) string {
	parts := make([]string, 0)
	for _, r := range arr {
		parts = append(parts, r.String())
	}
	return strings.Join(parts, ",")
}

func TestIntRange(t *testing.T) {
	tests := []struct {
		arr []int
		exp string
	}{
		{[]int{0}, "0"},
		{[]int{0, 1}, "0-1"},
		{[]int{5, 8, 7}, "5,7-8"},
		{[]int{28, 3, 2, 1, 4, 5, 8, 9}, "1-5,8-9,28"},
	}
	for _, test := range tests {
		r := IntRangeFromIntArray(test.arr)
		got := rangeArrToStr(r)
		if got != test.exp {
			t.Fatalf("for %#v got %s and expected %s", test.arr, got, test.exp)
		}
	}
}
