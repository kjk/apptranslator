// This code is under BSD license. See license-bsd.txt
package store

import (
	"sort"
	"strings"
)

func panicIf(shouldPanic bool, s string) {
	if shouldPanic {
		panic(s)
	}
}

type Translation struct {
	Id     int
	String string
	// last string is current translation, previous strings
	// are a history of how translation changed
	Translations []string
}

func NewTranslation(id int, s, trans string) *Translation {
	t := &Translation{Id: id, String: s}
	if trans != "" {
		t.add(trans)
	}
	return t
}

func (t *Translation) Current() string {
	n := len(t.Translations)
	if 0 == n {
		return ""
	}
	return t.Translations[n-1]
}

func (t *Translation) IsTranslated() bool {
	return len(t.Translations) > 0
}

func (t *Translation) History() []string {
	n := len(t.Translations)
	if n < 2 {
		return nil
	}
	return t.Translations[0 : n-1]
}

func (t *Translation) add(trans string) {
	t.Translations = append(t.Translations, trans)
}

const (
	stringCmpRemoveSet = ";,:()[]&_ "
)

func transStringLess(s1, s2 string) bool {
	s1 = strings.Trim(s1, stringCmpRemoveSet)
	s2 = strings.Trim(s2, stringCmpRemoveSet)
	s1 = strings.ToLower(s1)
	s2 = strings.ToLower(s2)
	return s1 < s2
}

type TranslationSeq []*Translation

func (s TranslationSeq) Len() int      { return len(s) }
func (s TranslationSeq) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type ByString struct{ TranslationSeq }

func (s ByString) Less(i, j int) bool {
	s1 := s.TranslationSeq[i].String
	s2 := s.TranslationSeq[j].String
	trans1 := s.TranslationSeq[i].Current()
	trans2 := s.TranslationSeq[j].Current()
	if trans1 == "" && trans2 != "" {
		return true
	}
	if trans2 == "" && trans1 != "" {
		return false
	}
	return transStringLess(s1, s2)
}

type LangInfo struct {
	Code          string
	Name          string
	ActiveStrings []*Translation
	UnusedStrings []*Translation
	untranslated  int
}

// for sorting by name
type LangInfoSeq []*LangInfo

func (s LangInfoSeq) Len() int      { return len(s) }
func (s LangInfoSeq) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type ByName struct{ LangInfoSeq }

func (s ByName) Less(i, j int) bool { return s.LangInfoSeq[i].Name < s.LangInfoSeq[j].Name }

// for sorting by untranslated count
type ByUntranslated struct{ LangInfoSeq }

func (s ByUntranslated) Less(i, j int) bool {
	l1 := s.LangInfoSeq[i].UntranslatedCount()
	l2 := s.LangInfoSeq[j].UntranslatedCount()
	if l1 != l2 {
		return l1 > l2
	}
	// to make sort more stable, we compare by name if counts are the same
	return s.LangInfoSeq[i].Name < s.LangInfoSeq[j].Name
}

func SortLangsByName(langs []*LangInfo) {
	sort.Sort(ByName{langs})
}

func NewLangInfo(langCode string) *LangInfo {
	li := &LangInfo{Code: langCode, Name: LangNameByCode(langCode), untranslated: -1}
	return li
}

func (li *LangInfo) UntranslatedCount() int {
	if li.untranslated == -1 {
		li.untranslated = 0
		for _, tr := range li.ActiveStrings {
			if !tr.IsTranslated() {
				li.untranslated += 1
			}
		}
	}
	return li.untranslated
}
