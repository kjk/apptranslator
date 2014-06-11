// This code is under BSD license. See license-bsd.txt
package store

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/kjk/u"
)

// Translation log is append-only file. It consists of records. Each record
// starts with varint-encoded (unsgined) length followed by data of that length.
// This allows us to detect and fix a corruption caused by not fully writing the
// last record (e.g. because of power failure in the middle of the write).
//
// In order to minimize the size of the file, we intern some strings by assigning
// them unique, monotonically increasing integers, e.g. we identify languages
// by their code names like "en", "pl" etc. Instead of repeating
//
// We have several types of records:
// - record interning a new language string, format:
//
//   byte(0) byte(1) string
//
// - record interning a new user name string, format:
//
//   byte(0) byte(2) string
//
// - record interning a new string for translation, format:
//
//   byte(0) byte(3) string
//
// - record denoting a given string has been "deleted" i.e. the app no longer
//   needs the translation of that string. We don't delete past translations
//   of strings, format:
//
//   byte(0) byte(4) varint(stringId)
//
// - record denoting a given string has been "undeleted", which is a faster
//   way to restore translations that to re-add them, format:
//
//   byte(0) byte(5) varint(stringId)
//
// - record defining a single translation, format:
//
//   varint(langId) varint(userId) varint(stringId) string
//
//   Note: varint(stringId) is guaranteed to not have 0 as first byte
//   which is why all other (much less frequent records) start with byte 0

const (
	newLangIdRec     = 1
	newUserNameIdRec = 2
	newStringIdRec   = 3
	strDelRec        = 4
	strUndelRec      = 5
)

var (
	logging              = false
	errorRecordMalformed = errors.New("Record malformed")
)

type TranslationRec struct {
	langId      int
	userId      int
	stringId    int
	translation string
	time        time.Time
}

type Edit struct {
	Lang        string
	User        string
	Text        string
	Translation string
	Time        time.Time
}

type Translator struct {
	Name              string
	TranslationsCount int
}

type StoreBinary struct {
	sync.Mutex
	langCodeMap    map[string]int
	userNameMap    map[string]int
	stringMap      map[string]int
	deletedStrings map[int]bool
	edits          []TranslationRec
	filePath       string
	w              io.Writer
	file           *os.File
}

// TODO: speed up by keeping all langs in array
func (s *StoreBinary) langById(langId int) string {
	for str, id := range s.langCodeMap {
		if id == langId {
			return str
		}
	}
	panic("didn't find the lang")
}

// TODO: speed up by keeping all users in array
func (s *StoreBinary) userById(userId int) string {
	for str, id := range s.userNameMap {
		if id == userId {
			return str
		}
	}
	panic("didn't find the user")
}

func stringById(stringMap map[string]int, strId int) string {
	for str, id := range stringMap {
		if id == strId {
			return str
		}
	}
	panic("didn't find the string")
}

func (s *StoreBinary) stringById(strId int) string {
	return stringById(s.stringMap, strId)
}

func (s *StoreBinary) recentEdits(max int) []Edit {
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

func (s *StoreBinary) editsByUser(user string) []Edit {
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

func (s *StoreBinary) editsForLang(lang string, max int) []Edit {
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

func (s *StoreBinary) translators() []*Translator {
	m := make(map[int]*Translator)
	unknownUserId := 1
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

func (s *StoreBinary) activeStringsCount() int {
	return len(s.stringMap) - len(s.deletedStrings)
}

func (s *StoreBinary) isDeleted(strId int) bool {
	_, exists := s.deletedStrings[strId]
	return exists
}

func (s *StoreBinary) getDeletedStrings() []string {
	res := make([]string, 0)
	for strId, _ := range s.deletedStrings {
		str := s.stringById(strId)
		res = append(res, str)
	}
	sort.Strings(res)
	return res
}

func (s *StoreBinary) getActiveStrings() []string {
	res := make([]string, 0)
	for str, strId := range s.stringMap {
		if !s.isDeleted(strId) {
			res = append(res, str)
		}
	}
	return res
}

func (s *StoreBinary) translatedCountForLangs() map[int]int {
	m := make(map[int][]bool)
	totalStrings := len(s.stringMap)
	for _, langId := range s.langCodeMap {
		arr := make([]bool, totalStrings)
		for i := 0; i < totalStrings; i++ {
			arr[i] = false
		}
		m[langId] = arr
	}
	res := make(map[int]int)
	for _, trec := range s.edits {
		if !s.isDeleted(trec.stringId) {
			arr := m[trec.langId]
			arr[trec.stringId-1] = true
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

func (s *StoreBinary) untranslatedCount() int {
	n := 0
	totalStrings := s.activeStringsCount()
	for _, translatedCount := range s.translatedCountForLangs() {
		n += (totalStrings - translatedCount)
	}
	return n
}

func (s *StoreBinary) untranslatedForLang(lang string) int {
	translatedPerLang := s.translatedCountForLangs()
	langId := s.langCodeMap[lang]
	translated := translatedPerLang[langId]
	return s.activeStringsCount() - translated
}

func (s *StoreBinary) validLangId(id int) bool {
	return (id >= 0) && (id <= len(s.langCodeMap))
}

func (s *StoreBinary) validUserId(id int) bool {
	return (id >= 0) && (id <= len(s.userNameMap))
}

func (s *StoreBinary) validStringId(id int) bool {
	return (id >= 0) && (id <= len(s.stringMap))
}

type ReaderByteReader struct {
	io.Reader
}

func (r *ReaderByteReader) ReadByte() (byte, error) {
	var buf [1]byte
	_, err := r.Read(buf[0:1])
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

func writeUvarintToBuf(b *bytes.Buffer, i int) {
	var buf [32]byte
	n := binary.PutUvarint(buf[:], uint64(i))
	b.Write(buf[:n]) // ignore error, it's always nil
}

func readUVarintAsInt(r *ReaderByteReader) (int, error) {
	n, err := binary.ReadUvarint(r)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func encodeRecordData(rec []byte, t time.Time) *bytes.Buffer {
	var b bytes.Buffer
	panicIf(0 == len(rec), "0 == recLen")
	writeUvarintToBuf(&b, len(rec)+4) // 4 for 64-bit tn
	var tn int64 = t.Unix()
	binary.Write(&b, binary.LittleEndian, tn)
	b.Write(rec) // // ignore error, it's always nil
	return &b
}

func writeRecord(w io.Writer, rec []byte, time time.Time) error {
	b := encodeRecordData(rec, time)
	_, err := w.Write(b.Bytes())
	return err
}

func writeStringInternRecord(buf *bytes.Buffer, recId int, s string) {
	var b bytes.Buffer
	b.WriteByte(0)
	b.WriteByte(byte(recId))
	b.WriteString(s)
	// ignoring error because writing to bytes.Buffer never fails
	writeRecord(buf, b.Bytes(), time.Now())
}

func writeDeleteStringRecord(w io.Writer, strId int) error {
	var b bytes.Buffer
	b.WriteByte(0)
	b.WriteByte(strDelRec)
	writeUvarintToBuf(&b, strId)
	return writeRecord(w, b.Bytes(), time.Now())
}

func writeUndeleteStringRecord(w io.Writer, strId int) error {
	var b bytes.Buffer
	b.WriteByte(0)
	b.WriteByte(strUndelRec)
	writeUvarintToBuf(&b, strId)
	return writeRecord(w, b.Bytes(), time.Now())
}

// returns a unique integer 1..n for a given string
// if the id hasn't been seen yet
func writeUniquifyStringRecord(buf *bytes.Buffer, s string, dict map[string]int, recId int) int {
	if n, ok := dict[s]; ok {
		return n
	}
	n := len(dict) + 1
	dict[s] = n
	writeStringInternRecord(buf, recId, s)
	return n
}

func (s *StoreBinary) addTranslationRec(langId, userId, stringId int, translation string, time time.Time) {
	t := &TranslationRec{langId, userId, stringId, translation, time}
	s.edits = append(s.edits, *t)
}

func (s *StoreBinary) deleteString(w io.Writer, str string) error {
	if strId, ok := s.stringMap[str]; !ok {
		log.Fatalf("deleteString() %q doesn't exist in stringMap\n", str)
	} else {
		if _, exists := s.deletedStrings[strId]; exists {
			fmt.Printf("deleteString: skipping deleting %d (%q) because already deleted", strId, str)
			return nil
		}
		if err := writeDeleteStringRecord(w, strId); err != nil {
			return err
		}
		s.deletedStrings[strId] = true
	}
	return nil
}

func (s *StoreBinary) undeleteString(w io.Writer, str string) error {
	if strId, ok := s.stringMap[str]; !ok {
		log.Fatalf("undeleteString() %q doesn't exist in stringMap\n", str)
	} else {
		if !s.isDeleted(strId) {
			log.Fatalf("undeleteString(): strId=%d doesn't exist in deletedStrings\n", strId)
		}
		if err := writeUndeleteStringRecord(w, strId); err != nil {
			return err
		}
		delete(s.deletedStrings, strId)
	}
	return nil
}

func (s *StoreBinary) duplicateTranslation(w io.Writer, origStr, newStr string) error {
	for _, edit := range s.edits {
		str := s.stringById(edit.stringId)
		if str != origStr {
			continue
		}
		lang := s.langById(edit.langId)
		user := s.userById(edit.userId)
		err := s.writeNewTranslation(w, newStr, edit.translation, lang, user)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *StoreBinary) writeNewTranslation(w io.Writer, txt, trans, lang, user string) error {
	var recs bytes.Buffer
	langId := writeUniquifyStringRecord(&recs, lang, s.langCodeMap, newLangIdRec)
	userId := writeUniquifyStringRecord(&recs, user, s.userNameMap, newUserNameIdRec)
	stringId := writeUniquifyStringRecord(&recs, txt, s.stringMap, newStringIdRec)

	var b bytes.Buffer
	writeUvarintToBuf(&b, langId)
	writeUvarintToBuf(&b, userId)
	writeUvarintToBuf(&b, stringId)
	b.Write([]byte(trans))

	t := time.Now()
	writeRecord(&recs, b.Bytes(), t) // cannot fail

	if _, err := w.Write(recs.Bytes()); err != nil {
		return err
	}
	s.addTranslationRec(langId, userId, stringId, trans, t)
	return nil
}

func (s *StoreBinary) writeNewStringRecord(w io.Writer, str string) error {
	var recs bytes.Buffer
	writeUniquifyStringRecord(&recs, str, s.stringMap, newStringIdRec)
	//fmt.Printf("%v\n", recs.Bytes())
	_, err := w.Write(recs.Bytes())
	if err != nil && logging {
		fmt.Printf("writeNewStringRecord() failed with %s\n", err)
	}
	return err
}

// rec is: string
func decodeStrRecord(rec []byte, dict map[string]int, tp string) error {
	str := string(rec)
	// id is inferred from the order, starts at 1
	id := len(dict) + 1
	if logging {
		fmt.Printf("decodeStrRecord(): %s -> %v in %s\n", str, id, tp)
	}
	if _, exists := dict[str]; exists {
		log.Fatalf("decodeStrRecord(): %q already exists in dict %s\n", str, tp)
	}
	dict[str] = id
	return nil
}

// rec is: varint(stringId)
func (s *StoreBinary) decodeStringDeleteRecord(rec []byte) error {
	id, n := binary.Uvarint(rec)
	panicIf(n != len(rec), "decodeStringDeleteRecord")
	//fmt.Printf("Deleting %d\n", int(id))
	if s.isDeleted(int(id)) {
		//log.Fatalf("decodeStringDeleteRecord(): '%d' already exists in deletedString\n", id)
		txt := s.stringById(int(id))
		fmt.Printf("decodeStringDeleteRecord(): %d (%q) already exists in deletedString\n", id, txt)
	}
	if logging {
		fmt.Printf("decodeStringDeleteRecord(): %d\n", id)
	}
	s.deletedStrings[int(id)] = true
	return nil
}

// rec is: varint(stringId)
func (s *StoreBinary) decodeStringUndeleteRecord(rec []byte) error {
	id, n := binary.Uvarint(rec)
	panicIf(n != len(rec), "decodeStringUndeleteRecord")
	if !s.isDeleted(int(id)) {
		//log.Fatalf("decodeStringUndeleteRecord(): '%d' doesn't exists in deletedStrings\n", id)
		fmt.Printf("decodeStringUndeleteRecord(): '%d' doesn't exists in deletedStrings\n", id)
	}
	if logging {
		fmt.Printf("decodeStringUndeleteRecord(): %d\n", id)
	}
	fmt.Printf("Undeleting %d\n", int(id))
	delete(s.deletedStrings, int(id))
	return nil
}

// rec is: varint(stringId) varint(userId) varint(langId) string
func (s *StoreBinary) decodeNewTranslation(rec []byte, time time.Time) error {
	var stringId, userId, langId uint64
	var n int

	langId, n = binary.Uvarint(rec)
	panicIf(n <= 0 || n == len(rec), "decodeNewTranslation() langId")
	panicIf(!s.validLangId(int(langId)), "decodeNewTranslation(): !s.validLangId()")
	rec = rec[n:]

	userId, n = binary.Uvarint(rec)
	panicIf(n == len(rec), "decodeNewTranslation() userId")
	panicIf(!s.validUserId(int(userId)), "decodeNewTranslation(): !s.validUserId()")
	rec = rec[n:]

	stringId, n = binary.Uvarint(rec)
	panicIf(n == 0 || n == len(rec), "decodeNewTranslation() stringId")
	panicIf(!s.validStringId(int(stringId)), fmt.Sprintf("decodeNewTranslation(): !s.validStringId(%v)", stringId))
	rec = rec[n:]

	translation := string(rec)
	s.addTranslationRec(int(langId), int(userId), int(stringId), translation, time)
	if logging {
		fmt.Printf("decodeNewTranslation(): %v, %v, %v, %s\n", langId, userId, stringId, translation)
	}

	if csvWriter != nil {
		timeSecsStr := strconv.FormatInt(time.Unix(), 10)

		stringStr := s.stringById(int(stringId))
		strId, newString := internedStrings.Intern(stringStr)
		if newString {
			recs := []string{recIdNewString, strconv.Itoa(strId), stringStr}
			writeCsv(recs)
		}

		langStr := s.langById(int(langId))
		userStr := s.userById(int(userId))
		recs := []string{recIdTrans, timeSecsStr, userStr, langStr, strconv.Itoa(strId), translation}
		writeCsv(recs)
	}
	return nil
}

func (s *StoreBinary) decodeRecord(rec []byte, time time.Time) error {
	panicIf(len(rec) < 2, "decodeRecord(), len(rec) < 2")
	if 0 == rec[0] {
		t := rec[1]
		switch t {
		case newLangIdRec:
			return decodeStrRecord(rec[2:], s.langCodeMap, "langCodeMap")
		case newUserNameIdRec:
			return decodeStrRecord(rec[2:], s.userNameMap, "userNameMap")
		case newStringIdRec:
			return decodeStrRecord(rec[2:], s.stringMap, "stringMap")
		case strDelRec:
			return s.decodeStringDeleteRecord(rec[2:])
		case strUndelRec:
			return s.decodeStringUndeleteRecord(rec[2:])
		default:
			log.Fatalf("Unexpected t=%d", t)
		}
	} else {
		return s.decodeNewTranslation(rec, time)
	}
	return nil
}

// TODO: optimize by passing in a buffer to fill instead of allocating a new
// one every time?
func readRecord(r *ReaderByteReader) ([]byte, time.Time, error) {
	var t time.Time
	n, err := readUVarintAsInt(r)
	if err != nil {
		if err == io.EOF {
			return nil, t, nil
		}
		log.Fatalf("readRecord(), err: %s", err)
	}
	panicIf(n <= 4, "record too small")
	var timeUnix int64
	err = binary.Read(r, binary.LittleEndian, &timeUnix)
	if err != nil {
		return nil, t, err
	}
	t = time.Unix(timeUnix, 0)
	buf := make([]byte, n-4)
	n, err = r.Read(buf)
	if err != nil {
		return nil, t, err
	}
	return buf, t, err
}

func (s *StoreBinary) readExistingRecords(r *ReaderByteReader) error {
	for {
		// TODO: not sure what to do about an error here
		//lastValidOffset, _ := s.file.Seek(1, 0)
		// TODO: could explicitly see if this is EOF by comparing lastValidOffset
		// with file size. that would simplify error handling
		buf, time, err := readRecord(r)
		if err != nil {
			// TODO: do something on error
			fmt.Printf("readExistingRecords(): error=%s\n", err)
			panic("readExistingRecords")
		}
		if buf == nil {
			return nil
		}
		err = s.decodeRecord(buf, time)
		if err != nil {
			return err
		}
	}
	panic("not reached")
	return nil
}

func (s *StoreBinary) translationsForLang(langId int) ([]*Translation, int) {
	// for speed, create an id -> string map once to avoid traversing stringMap
	// many times
	idToStr := make(map[int]string)
	for str, id := range s.stringMap {
		idToStr[id] = str
	}

	translations := make(map[string]*Translation)
	for _, edit := range s.edits {
		strId := edit.stringId
		if langId != edit.langId || s.isDeleted(strId) {
			continue
		}
		str := idToStr[strId]
		if tr, ok := translations[str]; ok {
			tr.add(edit.translation)
		} else {
			translations[str] = NewTranslation(strId, str, edit.translation)
		}
	}
	translatedCount := len(translations)
	// add records for untranslated strings
	for str, strId := range s.stringMap {
		if !s.isDeleted(strId) {
			if _, exists := translations[str]; !exists {
				translations[str] = &Translation{strId, str, make([]string, 0)}
			}
		}
	}
	res := make([]*Translation, len(translations))
	i := 0
	for _, t := range translations {
		res[i] = t
		i++
	}
	return res, s.activeStringsCount() - translatedCount
}

func (s *StoreBinary) langInfos() []*LangInfo {
	res := make([]*LangInfo, 0)
	for _, lang := range Languages {
		langCode := lang.Code
		li := NewLangInfo(langCode)
		langId := s.langCodeMap[langCode]
		li.ActiveStrings, _ = s.translationsForLang(langId)
		sort.Sort(ByString{li.ActiveStrings})
		res = append(res, li)
	}
	sort.Sort(ByUntranslated{res})
	return res
}

func NewStoreBinaryWithWriter(w io.Writer) (*StoreBinary, error) {
	s := &StoreBinary{
		langCodeMap:    make(map[string]int),
		userNameMap:    make(map[string]int),
		stringMap:      make(map[string]int),
		edits:          make([]TranslationRec, 0),
		deletedStrings: make(map[int]bool),
		w:              w,
	}
	return s, nil
}

func NewStoreBinary(path string) (*StoreBinary, error) {
	s := &StoreBinary{
		filePath:       path,
		langCodeMap:    make(map[string]int),
		userNameMap:    make(map[string]int),
		stringMap:      make(map[string]int),
		edits:          make([]TranslationRec, 0),
		deletedStrings: make(map[int]bool),
	}
	if u.PathExists(path) {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		err = s.readExistingRecords(&ReaderByteReader{file})
		file.Close()
		if err != nil {
			log.Fatalf("Failed to read log %q, err: %s\n", path, err)
		}
	} else {
		file, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		file.Close()
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("NewStoreBinary(): failed to open file %q, %s\n", path, err)
		return nil, err
	}
	s.file = file
	s.w = file
	return s, nil
}

func (s *StoreBinary) Close() {
	if s.file != nil {
		s.file.Close()
		s.file = nil
	}
}

func (s *StoreBinary) WriteNewTranslation(txt, trans, lang, user string) error {
	s.Lock()
	defer s.Unlock()
	return s.writeNewTranslation(s.w, txt, trans, lang, user)
}

func (s *StoreBinary) DuplicateTranslation(origStr, newStr string) error {
	s.Lock()
	defer s.Unlock()
	return s.duplicateTranslation(s.w, origStr, newStr)
}

func (s *StoreBinary) LangsCount() int {
	s.Lock()
	defer s.Unlock()
	return len(s.langCodeMap)
}

func (s *StoreBinary) StringsCount() int {
	s.Lock()
	defer s.Unlock()
	return s.activeStringsCount()
}

func (s *StoreBinary) EditsCount() int {
	return 0
}

func (s *StoreBinary) UntranslatedCount() int {
	s.Lock()
	defer s.Unlock()
	return s.untranslatedCount()
}

func (s *StoreBinary) UntranslatedForLang(lang string) int {
	s.Lock()
	defer s.Unlock()
	return s.untranslatedForLang(lang)
}

func (s *StoreBinary) LangInfos() []*LangInfo {
	s.Lock()
	defer s.Unlock()
	return s.langInfos()
}

func (s *StoreBinary) RecentEdits(max int) []Edit {
	s.Lock()
	defer s.Unlock()
	return s.recentEdits(max)
}

func (s *StoreBinary) EditsByUser(user string) []Edit {
	s.Lock()
	defer s.Unlock()
	return s.editsByUser(user)
}

func (s *StoreBinary) EditsForLang(user string, max int) []Edit {
	s.Lock()
	defer s.Unlock()
	return s.editsForLang(user, max)
}

func (s *StoreBinary) Translators() []*Translator {
	s.Lock()
	defer s.Unlock()
	return s.translators()
}

func (s *StoreBinary) UpdateStringsList(newStrings []string) ([]string, []string, []string, error) {
	s.Lock()
	defer s.Unlock()

	// for faster detection if string exists in newStrings, build a hash
	stringsHash := make(map[string]bool)
	for _, s := range newStrings {
		stringsHash[s] = true
	}

	var toUndelete []string
	for _, str := range newStrings {
		if strId, ok := s.stringMap[str]; ok {
			if s.isDeleted(strId) {
				toUndelete = append(toUndelete, str)
			}
		}
	}

	var toDelete []string
	for str, _ := range s.stringMap {
		if _, ok := stringsHash[str]; !ok {
			toDelete = append(toDelete, str)
		}
	}

	var toAdd []string
	for str, _ := range stringsHash {
		if _, ok := s.stringMap[str]; !ok {
			toAdd = append(toAdd, str)
		}
	}

	for _, str := range toUndelete {
		if err := s.undeleteString(s.w, str); err != nil {
			return nil, nil, nil, err
		}
	}
	for _, str := range toDelete {
		if err := s.deleteString(s.w, str); err != nil {
			return nil, nil, nil, err
		}
	}
	for _, str := range toAdd {
		if err := s.writeNewStringRecord(s.w, str); err != nil {
			return nil, nil, nil, err
		}
	}
	return toAdd, toDelete, toUndelete, nil
}

func (s *StoreBinary) GetUnusedStrings() []string {
	s.Lock()
	defer s.Unlock()
	return s.getDeletedStrings()
}
