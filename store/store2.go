// This code is under BSD license. See license-bsd.txt
package store

import (
	"os"
	"sync"

	"github.com/kjk/u"
)

type Store interface {
	Close()
	WriteNewTranslation(txt, trans, lang, user string) error
	DuplicateTranslation(origStr, newStr string) error
	LangsCount() int
	StringsCount() int
	UntranslatedCount() int
	UntranslatedForLang(lang string) int
	LangInfos() []*LangInfo
	RecentEdits(max int) []Edit
	EditsByUser(user string) []Edit
	EditsForLang(user string, max int) []Edit
	Translators() []*Translator
	UpdateStringsList(newStrings []string) ([]string, []string, []string, error)
	GetDeletedStrings() []string
}

const (
	transRecId = "tr"
)

type StringInterner struct {
	strings []string
	strToId map[string]int
}

func NewStringInterner() *StringInterner {
	return &StringInterner{
		strings: make([]string, 0),
		strToId: make(map[string]int),
	}
}

// returns
func (i *StringInterner) GetId(s string) (id int, newString bool) {
	if id, exists := i.strToId[s]; exists {
		return id, false
	} else {
		id = len(i.strings)
		i.strToId[s] = id
		i.strings = append(i.strings, s)
		return id, true
	}
}

type StoreCsv struct {
	sync.Mutex
	filePath string
	file     *os.File
	strings  *StringInterner
}

func NewStoreCsv(path string) (*StoreCsv, error) {
	s := &StoreCsv{
		filePath: path,
		strings:  NewStringInterner(),
	}
	if u.PathExists(path) {
		if err := s.readExistingRecords(path); err != nil {
			return nil, err
		}
	}
	if file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err != nil {
		return nil, err
	} else {
		s.file = file
	}
	return s, nil
}

func (s *StoreCsv) readExistingRecords(path string) error {
	panic("NYI")
	return nil
}

func (s *StoreCsv) Close() {
	panic("NYI")
}

func (s *StoreCsv) WriteNewTranslation(txt, trans, lang, user string) error {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) DuplicateTranslation(origStr, newStr string) error {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) LangsCount() int {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) StringsCount() int {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) UntranslatedCount() int {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) UntranslatedForLang(lang string) int {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) LangInfos() []*LangInfo {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) RecentEdits(max int) []Edit {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) EditsByUser(user string) []Edit {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) EditsForLang(user string, max int) []Edit {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) Translators() []*Translator {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) UpdateStringsList(newStrings []string) ([]string, []string, []string, error) {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) GetDeletedStrings() []string {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}
