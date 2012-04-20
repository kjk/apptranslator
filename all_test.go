package main

import (
	"testing"
)

func TestParseSum(t *testing.T) {
	name := "sum_strings.txt"
	lt, err := ParseFile(name)
	if err != nil {
		t.Errorf("Failed to parse %s, %s", name, err.Error())
	}
	if 16 != len(lt.Strings) || 16 != len(lt.Translations) {
		t.Errorf("Expected 16 strings, got %d", len(lt.Strings))
	}
	if lt.LangId != "cn" {
		t.Errorf("Expected lt.LangId to be 'cn', is: '%s'", lt.LangId)
	}
	if lt.LangNameEnglish != "Chinese Simplified" {
		t.Errorf("Expected lt.LangNameEnglish to be 'Chinese Simplified', is: '%s'", lt.LangNameEnglish)

	}
	if lt.LangNameNative != "简体中文" {
		t.Errorf("Expected lt.LangNameEnglish to be '简体中文', is: '%s'", lt.LangNameNative)
	}
}
