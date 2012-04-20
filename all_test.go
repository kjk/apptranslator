package main

import (
	"testing"
)

func TestParseSum(t *testing.T) {
	name := "sum_strings.txt"
	_, err := ParseFile(name)
	if err != nil {
		t.Errorf("Failed to parse %s, %s", name, err.Error())
	}
}
