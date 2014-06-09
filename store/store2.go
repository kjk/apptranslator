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
	if id < 0 || id >= len(i.strings) {
		//fmt.Printf("no id %d in i.strings of len %d\n", id, len(i.strings))
		return "", false
	}
	return i.strings[id], true
}

func (i *StringInterner) IdByStrMust(s string) int {
	if id, exists := i.strToId[s]; !exists {
		panic("s not in i.strToId")
	} else {
		return id
	}
}

func (i *StringInterner) Count() int {
	return len(i.strings)
}

type StoreCsv struct {
	sync.Mutex
	filePath      string
	file          *os.File
	strings       *StringInterner
	users         *StringInterner
	langs         *StringInterner
	w             *csv.Writer
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
		s.w = csv.NewWriter(s.file)
	}
	return s, nil
}

func (s *StoreCsv) writeCsv(rec []string) error {
	recs := [][]string{rec}
	return s.w.WriteAll(recs)
}

func (s *StoreCsv) writeNewStringRec(strId int, str string) error {
	recs := []string{recIdNewString, strconv.Itoa(strId), str}
	return s.writeCsv(recs)
}

func (s *StoreCsv) stringInternAndWriteIfNecessary(str string) (int, error) {
	if strId, isNew := s.strings.Intern(str); isNew {
		return strId, s.writeNewStringRec(strId, str)
	} else {
		return strId, nil
	}
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

func (s *StoreCsv) addTranslationRec(strId, langId, userId int, trans string, time time.Time) {
	tr := TranslationRec{
		langId:      langId,
		userId:      userId,
		stringId:    strId,
		translation: trans,
		time:        time,
	}
	s.edits = append(s.edits, tr)
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
	trans := rec[5]
	s.addTranslationRec(strId, langId, userId, trans, time)
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

func (s *StoreCsv) stringsCount() int {
	return len(s.activeStrings)
}

func (s *StoreCsv) translatedCountForLangs() map[int]int {
	m := make(map[int][]bool)
	totalStrings := s.strings.Count()
	for _, langId := range s.langs.strToId {
		m[langId] = make([]bool, totalStrings, totalStrings)
	}
	res := make(map[int]int)
	for _, trec := range s.edits {
		if !s.isDeleted(trec.stringId) {
			arr := m[trec.langId]
			arr[trec.stringId] = true
		}
	}
	for langId, arr := range m {
		count := 0
		for _, isTranslated := range arr {
			if isTranslated {
				count += 1
			}
		}
		res[langId] = count
	}
	return res
}

func (s *StoreCsv) untranslatedCount() int {
	n := 0
	totalStrings := s.stringsCount()
	for _, translatedCount := range s.translatedCountForLangs() {
		n += (totalStrings - translatedCount)
	}
	return n
}

func (s *StoreCsv) untranslatedForLang(lang string) int {
	translatedPerLang := s.translatedCountForLangs()
	langId := s.langs.IdByStrMust(lang)
	translated := translatedPerLang[langId]
	return s.stringsCount() - translated
}

func (s *StoreCsv) userById(id int) string {
	str, ok := s.users.GetById(id)
	panicIf(!ok, "no id in s.users")
	return str
}

func (s *StoreCsv) langById(id int) string {
	str, ok := s.langs.GetById(id)
	panicIf(!ok, "no id in s.langs")
	return str
}

func (s *StoreCsv) stringById(id int) string {
	//fmt.Printf("id: %d, total: %d\n", id, s.strings.Count())
	str, ok := s.strings.GetById(id)
	panicIf(!ok, "no id in s.strings")
	return str
}

func (s *StoreCsv) recentEdits(max int) []Edit {
	n := max
	transCount := len(s.edits)
	if n > transCount {
		n = transCount
	}
	res := make([]Edit, n, n)
	for i := 0; i < n; i++ {
		tr := &(s.edits[transCount-i-1])
		var e Edit
		e.Lang = s.langById(tr.langId)
		e.User = s.userById(tr.userId)
		e.Text = s.stringById(tr.stringId)
		e.Translation = tr.translation
		e.Time = tr.time
		res[i] = e
	}
	return res
}

func (s *StoreCsv) isDeleted(strId int) bool {
	// TODO: could speed up by building s.deletedStrings map[int]interface{}
	// that would be reconstructed when s.activeStrings is set
	for _, id := range s.activeStrings {
		if id == strId {
			return false
		}
	}
	return true
}

func (s *StoreCsv) translationsForLang(langId int) ([]*Translation, int) {
	translations := make(map[string]*Translation)
	for _, rec := range s.edits {
		if langId != rec.langId || s.isDeleted(rec.stringId) {
			continue
		}
		str, ok := s.strings.GetById(rec.stringId)
		panicIf(!ok, "rec.stringId should be in s.strings")
		if tr, ok := translations[str]; ok {
			tr.Translations = append(tr.Translations, rec.translation)
		} else {
			t := &Translation{str, make([]string, 1)}
			t.Translations[0] = rec.translation
			translations[str] = t
		}
	}
	translatedCount := len(translations)
	// add records for untranslated strings
	for str, strId := range s.strings.strToId {
		if !s.isDeleted(strId) {
			if _, exists := translations[str]; !exists {
				translations[str] = &Translation{str, make([]string, 0)}
			}
		}
	}
	res := make([]*Translation, len(translations))
	i := 0
	for _, t := range translations {
		res[i] = t
		i++
	}
	return res, s.stringsCount() - translatedCount
}

func (s *StoreCsv) langInfos() []*LangInfo {
	res := make([]*LangInfo, 0)
	for langCode, langId := range s.langs.strToId {
		li := NewLangInfo(langCode)
		li.Translations, li.untranslated = s.translationsForLang(langId)
		sort.Sort(ByString{li.Translations})
		res = append(res, li)
	}
	sort.Sort(ByUntranslated{res})
	return res
}

func (s *StoreCsv) editsByUser(user string) []Edit {
	res := make([]Edit, 0)
	transCount := len(s.edits)
	for i := 0; i < transCount; i++ {
		tr := &(s.edits[transCount-i-1])
		editUser := s.userById(tr.userId)
		if editUser == user {
			var e = Edit{
				Lang:        s.langById(tr.langId),
				User:        editUser,
				Text:        s.stringById(tr.stringId),
				Translation: tr.translation,
				Time:        tr.time,
			}
			res = append(res, e)
		}
	}
	return res
}

func (s *StoreCsv) editsForLang(lang string, max int) []Edit {
	res := make([]Edit, 0)
	transCount := len(s.edits)
	for i := 0; i < transCount; i++ {
		tr := &(s.edits[transCount-i-1])
		editLang := s.langById(tr.langId)
		if editLang == lang {
			var e = Edit{
				Lang:        s.langById(tr.langId),
				User:        s.userById(tr.userId),
				Text:        s.stringById(tr.stringId),
				Translation: tr.translation,
				Time:        tr.time,
			}
			res = append(res, e)
			if max != -1 && len(res) >= max {
				return res
			}
		}
	}
	return res
}

func (s *StoreCsv) translators() []*Translator {
	m := make(map[int]*Translator)
	unknownUserId := 0
	for _, tr := range s.edits {
		userId := tr.userId
		// filter out edits by the dummy 'unknown' user (used for translations
		// imported from the code before we had apptranslator)
		if userId == unknownUserId {
			continue
		}
		if t, ok := m[userId]; ok {
			t.TranslationsCount += 1
		} else {
			m[userId] = &Translator{Name: s.userById(userId), TranslationsCount: 1}
		}
	}
	n := len(m)
	res := make([]*Translator, n, n)
	i := 0
	for _, t := range m {
		res[i] = t
		i += 1
	}
	return res
}

func (s *StoreCsv) getDeletedStringsBitmap() []bool {
	n := s.strings.Count()
	res := make([]bool, n, n)
	for i := 0; i < n; i++ {
		res[i] = true
	}
	for _, id := range s.activeStrings {
		res[id] = false
	}
	return res
}

func (s *StoreCsv) getDeletedStrings() []string {
	deletedBitmap := s.getDeletedStringsBitmap()
	res := make([]string, 0)
	for strId, isDeleted := range deletedBitmap {
		if isDeleted {
			str := s.stringById(strId)
			res = append(res, str)
		}
	}
	sort.Strings(res)
	return res
}

// t,  ${timeUnix}, ${userStr}, ${langStr}, ${strId}, ${translation}
func (s *StoreCsv) writeNewTranslation(txt, trans, lang, user string) error {
	strId, err := s.stringInternAndWriteIfNecessary(txt)
	if err != nil {
		return err
	}
	langId, _ := s.langs.Intern(lang)
	userId, _ := s.users.Intern(user)
	t := time.Now()
	timeSecsStr := strconv.FormatInt(t.Unix(), 10)
	recs := []string{recIdTrans, timeSecsStr, user, lang, strconv.Itoa(strId), trans}
	if err = s.writeCsv(recs); err != nil {
		return err
	}
	s.addTranslationRec(strId, langId, userId, trans, t)
	return nil
}

func (s *StoreCsv) WriteNewTranslation(txt, trans, lang, user string) error {
	s.Lock()
	defer s.Unlock()
	return s.writeNewTranslation(txt, trans, lang, user)
}

func (s *StoreCsv) DuplicateTranslation(origStr, newStr string) error {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) LangsCount() int {
	s.Lock()
	defer s.Unlock()
	return s.langs.Count()
}

func (s *StoreCsv) StringsCount() int {
	s.Lock()
	defer s.Unlock()
	return s.stringsCount()
}

func (s *StoreCsv) UntranslatedCount() int {
	s.Lock()
	defer s.Unlock()
	return s.untranslatedCount()
}

func (s *StoreCsv) UntranslatedForLang(lang string) int {
	s.Lock()
	defer s.Unlock()
	return s.untranslatedForLang(lang)
}

func (s *StoreCsv) LangInfos() []*LangInfo {
	s.Lock()
	defer s.Unlock()
	return s.langInfos()
}

func (s *StoreCsv) RecentEdits(max int) []Edit {
	s.Lock()
	defer s.Unlock()
	return s.recentEdits(max)
}

func (s *StoreCsv) EditsByUser(user string) []Edit {
	s.Lock()
	defer s.Unlock()
	return s.editsByUser(user)
}

func (s *StoreCsv) EditsForLang(user string, max int) []Edit {
	s.Lock()
	defer s.Unlock()
	return s.editsForLang(user, max)
}

func (s *StoreCsv) Translators() []*Translator {
	s.Lock()
	defer s.Unlock()
	return s.translators()
}

func (s *StoreCsv) UpdateStringsList(newStrings []string) ([]string, []string, []string, error) {
	s.Lock()
	defer s.Unlock()
	panic("NYI")
}

func (s *StoreCsv) GetDeletedStrings() []string {
	s.Lock()
	defer s.Unlock()
	return s.getDeletedStrings()
}
