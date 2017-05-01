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

/* csv records:

s,  ${strId}, ${str}
t,  ${timeUnix}, ${userStr}, ${langStr}, ${strId}, ${translation}
as, ${timeUnix}, ${strId}, ...

*/
const (
	recIDNewString = "s"
	recIDTrans     = "t"
	recIDActiveSet = "as"
)

// TranslationRec represents translation record
type TranslationRec struct {
	langID      int
	userID      int
	stringID    int
	translation string
	time        time.Time
}

// Edit describes a single edit
type Edit struct {
	Lang        string
	User        string
	Text        string
	Translation string
	Time        time.Time
}

// Translator describes a translator
type Translator struct {
	Name              string
	TranslationsCount int
}

// StoreCsv describes a store for translations
type StoreCsv struct {
	sync.Mutex
	filePath             string
	file                 *os.File
	strings              *StringInterner
	users                *StringInterner
	w                    *csv.Writer
	activeStrings        []int
	deletedStringsBitmap []bool
	edits                []TranslationRec
}

func openCsv(path string) (*os.File, *csv.Writer, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, nil, err
	}
	return file, csv.NewWriter(file), nil
}

// NewStoreCsv creates new store using .csv for encoding
func NewStoreCsv(path string) (*StoreCsv, error) {
	//fmt.Printf("NewStoreCsv: %q\n", path)
	var err error
	s := &StoreCsv{
		filePath: path,
		strings:  NewStringInterner(),
		users:    NewStringInterner(),
		edits:    make([]TranslationRec, 0),
	}
	if u.PathExists(path) {
		if err = s.readExistingRecords(path); err != nil {
			return nil, err
		}
	}
	s.setActiveStrings(s.activeStrings)
	if s.file, s.w, err = openCsv(path); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *StoreCsv) writeCsv(rec []string) error {
	recs := [][]string{rec}
	return s.w.WriteAll(recs)
}

func (s *StoreCsv) writeNewStringRec(strID int, str string) error {
	rec := []string{recIDNewString, strconv.Itoa(strID), str}
	return s.writeCsv(rec)
}

func (s *StoreCsv) internStringAndWriteIfNecessary(str string) (int, error) {
	strID, isNew := s.strings.Intern(str)
	if isNew {
		//fmt.Printf("internStringAndWriteIfNecessary: new string %q, id: %d\n", str, strId)
		s.setActiveStrings(s.activeStrings)
		return strID, s.writeNewStringRec(strID, str)
	}
	//fmt.Printf("internStringAndWriteIfNecessary: existing string %q, id: %d\n", str, strId)
	return strID, nil
}

func buildActiveSetRec(activeStrings []int) []string {
	timeStr := strconv.FormatInt(time.Now().Unix(), 10)

	r := IntRangeFromIntArray(activeStrings)
	n := len(r)
	rec := make([]string, n+2, n+2)

	rec[0] = recIDActiveSet
	rec[1] = timeStr
	for i := 0; i < n; i++ {
		rec[2+i] = r[i].String()
	}
	return rec
}

// s,  ${strId}, ${str}
func (s *StoreCsv) decodeNewStringRecord(rec []string) error {
	if len(rec) != 3 {
		return fmt.Errorf("s record should have 3 fields, is '#%v'", rec)
	}
	id, err := strconv.Atoi(rec[1])
	if err != nil {
		return fmt.Errorf("rec[1] (%q) failed to parse as int with %q (rec: '%#v')", rec[1], err, rec)
	}
	newID, isNew := s.strings.Intern(rec[2])
	if newID != id {
		return fmt.Errorf("rec[2] is '%d' and we expect it to be '%d' (rec: '%#v')", id, newID, rec)
	}
	if !isNew {
		return fmt.Errorf("expected a new string in rec: '%#v')", rec)
	}
	return nil
}

func (s *StoreCsv) addTranslationRec(strID, langID, userID int, trans string, time time.Time) {
	if strID >= s.allStringsCount() {
		panic(fmt.Sprintf("strId >= s.allStringsCount() (%d >= %d)", strID, s.allStringsCount()))
	}
	tr := TranslationRec{
		langID:      langID,
		userID:      userID,
		stringID:    strID,
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
		return fmt.Errorf("rec[1] (%q) failed to parse as int64, error: %q", rec[1], err)
	}
	time := time.Unix(timeSecs, 0)
	userID, _ := s.users.Intern(rec[2])
	langID := LangToId(rec[3])
	fatalIf(langID < 0, "invalid rec: %#v", rec)
	strID, err := strconv.Atoi(rec[4])
	if err != nil {
		return fmt.Errorf("rec[4] (%q) failed to parse as int, error: %q", rec[4], err)
	}
	if _, ok := s.strings.GetById(strID); !ok {
		return fmt.Errorf("rec[4] (%q, '%d') is not a valid string id", rec[4], strID)
	}
	trans := rec[5]
	s.addTranslationRec(strID, langID, userID, trans, time)
	return nil
}

// as, ${timeUnix}, ${strId}, ...
func (s *StoreCsv) decodeActiveSetRecord(rec []string) error {
	if len(rec) < 3 {
		fmt.Printf("'as' record should have at least 3 fields, is '%#v'", rec)
	}
	n := len(rec) - 2
	activeRange := make([]IntRange, n, n)
	for i := 0; i < n; i++ {
		ir, err := ParseIntRange(rec[2+i])
		if err != nil {
			return fmt.Errorf("rec[%d] (%q) didn't parse as range, error: %q", 2+i, rec[2+i], err)
		}
		activeRange[i] = ir
	}

	s.setActiveStrings(IntRangeToArray(activeRange))
	return nil
}

func (s *StoreCsv) decodeRecord(rec []string) error {
	if len(rec) < 2 {
		return fmt.Errorf("not enough fields (%d) in %#v", len(rec), rec)
	}
	var err error
	switch rec[0] {
	case recIDNewString:
		err = s.decodeNewStringRecord(rec)
	case recIDActiveSet:
		err = s.decodeActiveSetRecord(rec)
	case recIDTrans:
		err = s.decodeTranslationRecord(rec)
	default:
		err = fmt.Errorf("unkown record type %q", rec[0])
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

// Close closes the store
func (s *StoreCsv) Close() {
	s.w.Flush()
	s.file.Close()
	s.file = nil
}

func (s *StoreCsv) allStringsCount() int {
	return s.strings.Count()
}

func (s *StoreCsv) activeStringsCount() int {
	return len(s.activeStrings)
}

func (s *StoreCsv) translatedCountForLangs() map[int]int {
	m := make(map[int][]bool)
	totalStrings := s.strings.Count()
	for langID := 0; langID < LangsCount(); langID++ {
		m[langID] = make([]bool, totalStrings, totalStrings)
	}
	res := make(map[int]int)
	for _, trec := range s.edits {
		if !s.isUnused(trec.stringID) {
			arr := m[trec.langID]
			arr[trec.stringID] = true
		}
	}
	for langID, arr := range m {
		count := 0
		for _, isTranslated := range arr {
			if isTranslated {
				count++
			}
		}
		res[langID] = count
	}
	return res
}

func (s *StoreCsv) untranslatedCount() int {
	n := 0
	totalStrings := s.activeStringsCount()
	for _, translatedCount := range s.translatedCountForLangs() {
		n += (totalStrings - translatedCount)
	}
	return n
}

func (s *StoreCsv) untranslatedForLang(lang string) int {
	translatedPerLang := s.translatedCountForLangs()
	langID := LangToId(lang)
	fatalIf(langID == -1, "LangToId(lang) returned -1")
	translated := translatedPerLang[langID]
	return s.activeStringsCount() - translated
}

func (s *StoreCsv) userByID(id int) string {
	str, ok := s.users.GetById(id)
	fatalIf(!ok, "no id in s.users")
	return str
}

func (s *StoreCsv) langByID(id int) string {
	langCode := LangCodeById(id)
	fatalIf(langCode == "", "LangCodeById(id) didn't find a lang")
	return langCode
}

func (s *StoreCsv) stringByIDMust(id int) string {
	//fmt.Printf("id: %d, total: %d\n", id, s.strings.Count())
	str, ok := s.strings.GetById(id)
	fatalIf(!ok, "no id in s.strings")
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
		e.Lang = s.langByID(tr.langID)
		e.User = s.userByID(tr.userID)
		e.Text = s.stringByIDMust(tr.stringID)
		e.Translation = tr.translation
		e.Time = tr.time
		res[i] = e
	}
	return res
}

func (s *StoreCsv) isUnused(strID int) bool {
	if strID >= s.allStringsCount() {
		fmt.Printf("strId %d too large, all strings: %d, bitmap len: %d\n", strID, s.allStringsCount(), len(s.deletedStringsBitmap))
	}
	if strID >= len(s.deletedStringsBitmap) {
		fmt.Printf("strId %d too large, all strings: %d, bitmap len: %d\n", strID, s.allStringsCount(), len(s.deletedStringsBitmap))
	}
	return s.deletedStringsBitmap[strID]
}

func (s *StoreCsv) translationsForLang(langID int) ([]*Translation, []*Translation) {
	n := len(s.strings.strings)
	all := make([]*Translation, n)
	for strID, str := range s.strings.strings {
		all[strID] = NewTranslation(strID, str, "")
	}

	for _, edit := range s.edits {
		if langID != edit.langID {
			continue
		}
		tr := all[edit.stringID]
		tr.add(edit.translation)
	}

	active := make([]*Translation, 0)
	unused := make([]*Translation, 0)
	for _, tr := range all {
		if s.isUnused(tr.Id) {
			unused = append(unused, tr)
		} else {
			active = append(active, tr)
		}
	}
	return active, unused
}

func (s *StoreCsv) langInfos() []*LangInfo {
	res := make([]*LangInfo, 0)
	for langID, lang := range Languages {
		langCode := lang.Code
		li := NewLangInfo(langCode)
		li.ActiveStrings, li.UnusedStrings = s.translationsForLang(langID)
		sort.Sort(ByString{li.ActiveStrings})
		sort.Sort(ByString2{li.UnusedStrings})
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
		editUser := s.userByID(tr.userID)
		if editUser == user {
			var e = Edit{
				Lang:        s.langByID(tr.langID),
				User:        editUser,
				Text:        s.stringByIDMust(tr.stringID),
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
		editLang := s.langByID(tr.langID)
		if editLang == lang {
			var e = Edit{
				Lang:        s.langByID(tr.langID),
				User:        s.userByID(tr.userID),
				Text:        s.stringByIDMust(tr.stringID),
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
	unknownUserID := 0
	for _, tr := range s.edits {
		userID := tr.userID
		// filter out edits by the dummy 'unknown' user (used for translations
		// imported from the code before we had apptranslator)
		if userID == unknownUserID {
			continue
		}
		if t, ok := m[userID]; ok {
			t.TranslationsCount++
		} else {
			m[userID] = &Translator{Name: s.userByID(userID), TranslationsCount: 1}
		}
	}
	n := len(m)
	res := make([]*Translator, n, n)
	i := 0
	for _, t := range m {
		res[i] = t
		i++
	}
	return res
}

func (s *StoreCsv) setActiveStrings(activeStrings []int) {
	if nil == activeStrings {
		activeStrings = make([]int, 0)
	}
	s.activeStrings = activeStrings
	n := s.allStringsCount()
	bitmap := make([]bool, n, n)
	for i := 0; i < n; i++ {
		bitmap[i] = true
	}
	for _, id := range s.activeStrings {
		bitmap[id] = false
	}
	s.deletedStringsBitmap = bitmap
	//fmt.Printf("setActiveStrings: n1: %d, n2: %d\n", n, len(s.deletedStringsBitmap))
}

func (s *StoreCsv) getDeletedStrings() []string {
	res := make([]string, 0)
	for strID, isDeleted := range s.deletedStringsBitmap {
		if isDeleted {
			str := s.stringByIDMust(strID)
			res = append(res, str)
		}
	}
	sort.Strings(res)
	return res
}

// t,  ${timeUnix}, ${userStr}, ${langStr}, ${strId}, ${translation}
func (s *StoreCsv) writeNewTranslation(txt, trans, lang, user string) error {
	strID, err := s.internStringAndWriteIfNecessary(txt)
	if err != nil {
		return err
	}
	langID := LangToId(lang)
	fatalIf(langID < 0, "invalid lang: %s", lang)
	userID, _ := s.users.Intern(user)
	t := time.Now()
	timeSecsStr := strconv.FormatInt(t.Unix(), 10)
	recs := []string{recIDTrans, timeSecsStr, user, lang, strconv.Itoa(strID), trans}
	if err = s.writeCsv(recs); err != nil {
		return err
	}
	s.addTranslationRec(strID, langID, userID, trans, t)
	return nil
}

func (s *StoreCsv) duplicateTranslation(origStr, newStr string) error {
	origStrID := s.strings.IdByStrMust(origStr)
	// find most recent translations for each language
	nLangs := LangsCount()
	langTrans := make([]string, nLangs, nLangs)
	langUserID := make([]int, nLangs, nLangs)
	for _, edit := range s.edits {
		if origStrID != edit.stringID {
			continue
		}
		langTrans[edit.langID] = edit.translation
		langUserID[edit.langID] = edit.userID
	}

	for langID, translation := range langTrans {
		if translation == "" {
			continue
		}
		lang := s.langByID(langID)
		user := s.userByID(langUserID[langID])
		trans := langTrans[langID]
		if err := s.writeNewTranslation(newStr, trans, lang, user); err != nil {
			return err
		}
	}

	return nil
}

// WriteNewTranslation writes new translation
func (s *StoreCsv) WriteNewTranslation(txt, trans, lang, user string) error {
	s.Lock()
	defer s.Unlock()
	return s.writeNewTranslation(txt, trans, lang, user)
}

// DuplicateTranslation duplicates a translation
func (s *StoreCsv) DuplicateTranslation(origStr, newStr string) error {
	s.Lock()
	defer s.Unlock()
	return s.duplicateTranslation(origStr, newStr)
}

// LangsCount returns number of langauges
func (s *StoreCsv) LangsCount() int {
	return LangsCount()
}

// StringsCount returns number of phrases
func (s *StoreCsv) StringsCount() int {
	s.Lock()
	defer s.Unlock()
	return s.activeStringsCount()
}

// EditsCount returns total number of edits
func (s *StoreCsv) EditsCount() int {
	s.Lock()
	defer s.Unlock()
	return len(s.edits)
}

// UntranslatedCount return number of untranslated phrases
func (s *StoreCsv) UntranslatedCount() int {
	s.Lock()
	defer s.Unlock()
	return s.untranslatedCount()
}

// UntranslatedForLang returns number of untranslated phrases for a given language
func (s *StoreCsv) UntranslatedForLang(lang string) int {
	s.Lock()
	defer s.Unlock()
	return s.untranslatedForLang(lang)
}

// LangInfos returns info about all languages
func (s *StoreCsv) LangInfos() []*LangInfo {
	s.Lock()
	defer s.Unlock()
	return s.langInfos()
}

// RecentEdits returns recent edits
func (s *StoreCsv) RecentEdits(max int) []Edit {
	s.Lock()
	defer s.Unlock()
	return s.recentEdits(max)
}

// EditsByUser returns edits by a given user
func (s *StoreCsv) EditsByUser(user string) []Edit {
	s.Lock()
	defer s.Unlock()
	return s.editsByUser(user)
}

// EditsForLang returns edits for a given language
func (s *StoreCsv) EditsForLang(user string, max int) []Edit {
	s.Lock()
	defer s.Unlock()
	return s.editsForLang(user, max)
}

// Translators returns all translators
func (s *StoreCsv) Translators() []*Translator {
	s.Lock()
	defer s.Unlock()
	return s.translators()
}

func (s *StoreCsv) writeActiveStringsRec(activeStrings []int) error {
	rec := buildActiveSetRec(activeStrings)
	return s.writeCsv(rec)
}

func (s *StoreCsv) writeActiveStrings(activeStrings []string) (err error) {
	n := len(activeStrings)
	activeStrIds := make([]int, n, n)
	for i, str := range activeStrings {
		activeStrIds[i], err = s.internStringAndWriteIfNecessary(str)
		if err != nil {
			return err
		}
	}
	if err = s.writeActiveStringsRec(activeStrIds); err != nil {
		return err
	}
	s.setActiveStrings(activeStrIds)
	return nil
}

// UpdateStringsList updates list of phrases
func (s *StoreCsv) UpdateStringsList(newStrings []string) ([]string, []string, []string, error) {
	s.Lock()
	defer s.Unlock()
	err := s.writeActiveStrings(newStrings)
	return nil, nil, nil, err
}

// GetUnusedStrings returns unused phrases
func (s *StoreCsv) GetUnusedStrings() []string {
	s.Lock()
	defer s.Unlock()
	return s.getDeletedStrings()
}
