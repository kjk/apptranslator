package store

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type IntRange struct {
	start int
	end   int
}

func NewIntRange(start, end int) IntRange {
	return IntRange{start, end}
}

func (r IntRange) String() string {
	if r.start == r.end {
		return strconv.Itoa(r.start)
	}
	return fmt.Sprintf("%d-%d", r.start, r.end)
}

func ParseIntRange(s string) (r IntRange, err error) {
	parts := strings.Split(s, "-")
	if len(parts) > 2 {
		return r, fmt.Errorf("'%s' is not a valid int range", s)
	}
	i1, err := strconv.Atoi(parts[0])
	if err != nil {
		return r, err
	}
	if len(parts) == 1 {
		return NewIntRange(i1, i1), nil
	}
	i2, err := strconv.Atoi(parts[1])
	if err != nil {
		return r, err
	}
	return NewIntRange(i1, i2), nil
}

func IntRangeFromIntArray(arr []int) []IntRange {
	sort.Ints(arr)
	res := make([]IntRange, 0)
	if len(arr) == 0 {
		return res
	}
	start := arr[0]
	end := start
	for _, n := range arr[1:] {
		if n == end+1 {
			end = n
			continue
		}
		r := NewIntRange(start, end)
		res = append(res, r)
		start = n
		end = start
	}
	r := NewIntRange(start, end)
	res = append(res, r)
	return res
}

func IntRangeToArray(r []IntRange) []int {
	res := make([]int, 0)
	for _, el := range r {
		for n := el.start; n <= el.end; n++ {
			res = append(res, n)
		}
	}
	sort.Ints(res)
	return res
}
