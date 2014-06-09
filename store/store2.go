// This code is under BSD license. See license-bsd.txt
package store

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

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

/* csv records:

s,  ${strId}, ${str}
t,  ${timeUnix}, ${userStr}, ${langStr}, ${strId}, ${translation}
as, ${timeUnix}, ${strId}, ...

*/
const (
	recIdNewString = "s"
	recIdTrans     = "t"
	recIdActiveSet = "as"
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

// returns existing id of the string and false if this string has been interned
// before. returns a newly allocated id for the string and true if this is
// a newly interned string
func (i *StringInterner) Intern(s string) (id int, isNew bool) {
	if id, exists := i.strToId[s]; exists {
		return id, false
	} else {
		id = len(i.strings)
		i.strToId[s] = id
		i.strings = append(i.strings, s)
		return id, true
	}
}

func (i *StringInterner) GetById(id int) (string, bool) {
	if id < 0 || id > len(i.strings) {
		return "", false
	}
	return i.strings[id], true
}

type StoreCsv struct {
	sync.Mutex
	filePath      string
	file          *os.File
	strings       *StringInterner
	users         *StringInterner
	langs         *StringInterner
	activeStrings []int
	edits         []TranslationRec
}

func NewStoreCsv(path string) (*StoreCsv, error) {
	fmt.Printf("NewStoreCsv: '%s'\n", path)
	s := &StoreCsv{
		filePath: path,
		strings:  NewStringInterner(),
		users:    NewStringInterner(),
		langs:    NewStringInterner(),
		edits:    make([]TranslationRec, 0),
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

// s,  ${strId}, ${str}
func (s *StoreCsv) decodeNewStringRecord(rec []string) error {
	if len(rec) != 3 {
		return fmt.Errorf("s record should have 3 fields, is '#%v'", rec)
	}
	id, err := strconv.Atoi(rec[1])
	if err != nil {
		return fmt.Errorf("rec[1] failed to parse as int with '%s' (rec: '%#v')", err, rec[1], rec)
	}
	newId, isNew := s.strings.Intern(rec[2])
	if newId != id {
		return fmt.Errorf("rec[2] is '%d' and we expect it to be '%d' (rec: '%#v')", id, newId, rec)
	}
	if !isNew {
		return fmt.Errorf("expected a new string in rec: '%#v')", rec)
	}
	return nil
}

// t,  ${timeUnix}, ${userStr}, ${langStr}, ${strId}, ${translation}
func (s *StoreCsv) decodeTranslationRecord(rec []string) error {
	if len(rec) != 6 {
		return fmt.Errorf("'t' record should have 6 fields, is '%#v'", rec)
	}
	timeSecs, err := strconv.ParseInt(rec[1], 10, 64)
	if err != nil {
		return fmt.Errorf("rec[1] ('%s') failed to parse as int64, error: '%s'", rec[1], err)
	}
	time := time.Unix(timeSecs, 0)
	userId, _ := s.users.Intern(rec[2])
	langId, _ := s.langs.Intern(rec[3])
	strId, err := strconv.Atoi(rec[4])
	if err != nil {
		return fmt.Errorf("rec[4] ('%s') failed to parse as int, error: '%s'", rec[4], err)
	}
	if _, ok := s.strings.GetById(strId); !ok {
		return fmt.Errorf("rec[4] ('%s', '%d') is not a valid string id", rec[4], strId)
	}
	tr := TranslationRec{
		langId:      langId,
		userId:      userId,
		stringId:    strId,
		translation: rec[5],
		time:        time,
	}
	s.edits = append(s.edits, tr)
	return nil
}

// as, ${timeUnix}, ${strId}, ...
func (s *StoreCsv) decodeActiveSetRecord(rec []string) error {
	if len(rec) < 3 {
		fmt.Printf("'as' record should have at least 3 fields, is '%#v'", rec)
	}
	n := len(rec) - 2
	active := make([]int, n, n)
	for i := 0; i < n; i++ {
		strId, err := strconv.Atoi(rec[2+i])
		if err != nil {
			return fmt.Errorf("rec[%d] ('%s') didn't parse as int, error: '%s'", 2+i, rec[2+i], err)
		}
		active[i] = strId
	}
	sort.Ints(active)
	s.activeStrings = active
	return nil
}

func (s *StoreCsv) decodeRecord(rec []string) error {
	if len(rec) < 2 {
		return fmt.Errorf("not enough fields (%d) in %#v", len(rec), rec)
	}
	var err error
	switch rec[0] {
	case recIdNewString:
		err = s.decodeNewStringRecord(rec)
	case recIdActiveSet:
		err = s.decodeActiveSetRecord(rec)
	case recIdTrans:
		err = s.decodeTranslationRecord(rec)
	default:
		err = fmt.Errorf("unkown record type '%s'", rec[0])
	}
	return err
}

func (s *StoreCsv) readExistingRecords(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.Comma = ','
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if err = s.decodeRecord(rec); err != nil {
			break
		}
	}
	if err == io.EOF {
		err = nil
	}
	return err
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
