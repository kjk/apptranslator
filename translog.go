package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
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
	logging = false
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
}

type Translator struct {
	Name              string
	TranslationsCount int
}

type EncoderDecoderState struct {
	langCodeMap    map[string]int
	userNameMap    map[string]int
	stringMap      map[string]int
	deletedStrings map[int]bool
	translations   []TranslationRec
}

// TODO: speed up by keeping all langs in array
func (s *EncoderDecoderState) langById(langId int) string {
	for str, id := range s.langCodeMap {
		if id == langId {
			return str
		}
	}
	panic("didn't find the lang")
}

// TODO: speed up by keeping all users in array
func (s *EncoderDecoderState) userById(userId int) string {
	for str, id := range s.userNameMap {
		if id == userId {
			return str
		}
	}
	panic("didn't find the user")
}

func (s *EncoderDecoderState) stringById(strId int) string {
	for str, id := range s.stringMap {
		if id == strId {
			return str
		}
	}
	panic("didn't find the string")
}

func (s *EncoderDecoderState) recentEdits(max int) []Edit {
	n := max
	transCount := len(s.translations)
	if n > transCount {
		n = transCount
	}
	res := make([]Edit, n, n)
	for i := 0; i < n; i++ {
		tr := &(s.translations[transCount-i-1])
		var e Edit
		e.Lang = s.langById(tr.langId)
		e.User = s.userById(tr.userId)
		e.Text = s.stringById(tr.stringId)
		e.Translation = tr.translation
		res[i] = e
	}
	return res
}

func (s *EncoderDecoderState) translators() []*Translator {
	m := make(map[int]*Translator)
	unknownUserId := 1
	for _, tr := range s.translations {
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

func (s *EncoderDecoderState) langsCount() int {
	return len(s.langCodeMap)
}

func (s *EncoderDecoderState) stringsCount() int {
	return len(s.stringMap) - len(s.deletedStrings)
}

func (s *EncoderDecoderState) isDeleted(strId int) bool {
	_, exists := s.deletedStrings[strId]
	return exists
}

func (s *EncoderDecoderState) translatedCountForLangs() map[int]int {
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
	for _, trec := range s.translations {
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

func (s *EncoderDecoderState) untranslatedCount() int {
	n := 0
	totalStrings := s.stringsCount()
	for _, translatedCount := range s.translatedCountForLangs() {
		n += (totalStrings - translatedCount)
	}
	return n
}

func (s *EncoderDecoderState) validLangId(id int) bool {
	return (id >= 0) && (id <= len(s.langCodeMap))
}

func (s *EncoderDecoderState) validUserId(id int) bool {
	return (id >= 0) && (id <= len(s.userNameMap))
}

func (s *EncoderDecoderState) validStringId(id int) bool {
	return (id >= 0) && (id <= len(s.stringMap))
}

type TranslationLog struct {
	state    *EncoderDecoderState
	filePath string
	file     *os.File

	// protects writing translations
	mu sync.Mutex
}

var errorRecordMalformed = errors.New("Record malformed")

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

func panicIf(shouldPanic bool, s string) {
	if shouldPanic {
		panic(s)
	}
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

func (s *EncoderDecoderState) addTranslationRec(langId, userId, stringId int, translation string, time time.Time) {
	t := &TranslationRec{langId, userId, stringId, translation, time}
	s.translations = append(s.translations, *t)
}

func (s *EncoderDecoderState) deleteString(w io.Writer, str string) error {
	if strId, ok := s.stringMap[str]; !ok {
		log.Fatalf("deleteString() '%s' doesn't exist in stringMap\n", str)
	} else {
		if err := writeDeleteStringRecord(w, strId); err != nil {
			return err
		}
		s.deletedStrings[strId] = true
	}
	return nil
}

func (s *EncoderDecoderState) undeleteString(w io.Writer, str string) error {
	if strId, ok := s.stringMap[str]; !ok {
		log.Fatalf("undeleteString() '%s' doesn't exist in stringMap\n", str)
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

func (s *EncoderDecoderState) writeNewTranslation(w io.Writer, txt, trans, lang, user string) error {
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

func (s *EncoderDecoderState) writeNewStringRecord(w io.Writer, str string) error {
	var recs bytes.Buffer
	writeUniquifyStringRecord(&recs, str, s.stringMap, newStringIdRec)
	//fmt.Printf("%v\n", recs.Bytes())
	_, err := w.Write(recs.Bytes())
	if err != nil && logging {
		fmt.Printf("writeNewStringRecord() failed with %s\n", err.Error())
	}
	return err
}

func NewEncoderDecoderState() *EncoderDecoderState {
	s := &EncoderDecoderState{
		langCodeMap:    make(map[string]int),
		userNameMap:    make(map[string]int),
		stringMap:      make(map[string]int),
		translations:   make([]TranslationRec, 0),
		deletedStrings: make(map[int]bool),
	}
	return s
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
		log.Fatalf("decodeStrRecord(): '%s' already exists in dict %s\n", str, tp)
	}
	dict[str] = id
	return nil
}

// rec is: varint(stringId)
func (s *EncoderDecoderState) decodeStringDeleteRecord(rec []byte) error {
	id, n := binary.Uvarint(rec)
	panicIf(n != len(rec), "decodeStringDeleteRecord")
	if s.isDeleted(int(id)) {
		log.Fatalf("decodeStringDeleteRecord(): '%d' already exists in deletedString\n", id)
	}
	if logging {
		fmt.Printf("decodeStringDeleteRecord(): %d\n", id)
	}
	s.deletedStrings[int(id)] = true
	return nil
}

// rec is: varint(stringId)
func (s *EncoderDecoderState) decodeStringUndeleteRecord(rec []byte) error {
	id, n := binary.Uvarint(rec)
	panicIf(n != len(rec), "decodeStringUndeleteRecord")
	if !s.isDeleted(int(id)) {
		log.Fatalf("decodeStringUndeleteRecord(): '%d' doesn't exists in deletedStrings\n", id)
	}
	if logging {
		fmt.Printf("decodeStringUndeleteRecord(): %d\n", id)
	}
	delete(s.deletedStrings, int(id))
	return nil
}

// rec is: varint(stringId) varint(userId) varint(langId) string
func (s *EncoderDecoderState) decodeNewTranslation(rec []byte, time time.Time) error {
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
	return nil
}

func (s *EncoderDecoderState) decodeRecord(rec []byte, time time.Time) error {
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
		log.Fatalf("readRecord(), err: %s", err.Error())
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

func (s *EncoderDecoderState) readExistingRecords(r *ReaderByteReader) error {
	for {
		// TODO: not sure what to do about an error here
		//lastValidOffset, _ := l.file.Seek(1, 0)
		// TODO: could explicitly see if this is EOF by comparing lastValidOffset
		// with file size. that would simplify error handling
		buf, time, err := readRecord(r)
		if err != nil {
			// TODO: do something on error
			fmt.Printf("readExistingRecords(): error=%s\n", err.Error())
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

type Translation struct {
	String string
	// last string is current translation, previous strings
	// are a history of how translation changed
	Translations []string
}

func NewTranslation(s, trans string) *Translation {
	t := &Translation{String: s}
	if trans != "" {
		t.Translations = append(t.Translations, trans)
	}
	return t
}

func (t *Translation) Current() string {
	if 0 == len(t.Translations) {
		return ""
	}
	return t.Translations[len(t.Translations)-1]
}

func (t *Translation) updateTranslation(trans string) {
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
	Code         string
	Name         string
	Translations []*Translation
	untranslated int
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

func NewLangInfo(langCode string) *LangInfo {
	li := &LangInfo{Code: langCode, Name: LangNameByCode(langCode)}
	return li
}

func (li *LangInfo) UntranslatedCount() int {
	return li.untranslated
}

func (s *EncoderDecoderState) translationsForLang(langId int) ([]*Translation, int) {
	// for speed, create an id -> string map once to avoid traversing stringMap
	// many times
	idToStr := make(map[int]string)
	for str, id := range s.stringMap {
		idToStr[id] = str
	}

	translations := make(map[string]*Translation)
	for _, rec := range s.translations {
		if langId != rec.langId || s.isDeleted(rec.stringId) {
			continue
		}
		str := idToStr[rec.stringId]
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
	for str, strId := range s.stringMap {
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

func (s *EncoderDecoderState) langInfos() []*LangInfo {
	res := make([]*LangInfo, 0)
	for langCode, langId := range s.langCodeMap {
		li := NewLangInfo(langCode)
		li.Translations, li.untranslated = s.translationsForLang(langId)
		sort.Sort(ByString{li.Translations})
		res = append(res, li)
	}
	sort.Sort(ByUntranslated{res})
	return res
}

func NewTranslationLog(path string) (*TranslationLog, error) {
	state := NewEncoderDecoderState()
	if FileExists(path) {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		err = state.readExistingRecords(&ReaderByteReader{file})
		file.Close()
		if err != nil {
			log.Fatalf("Failed to read log '%s', err: %s\n", path, err.Error())
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
		fmt.Printf("NewTranslationLog(): failed to open file '%s', %s\n", path, err.Error())
		return nil, err
	}
	l := &TranslationLog{filePath: path, file: file}
	l.state = state
	return l, nil
}

func (l *TranslationLog) close() {
	l.file.Close()
	l.file = nil
}

func (l *TranslationLog) writeNewTranslation(txt, trans, lang, user string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state.writeNewTranslation(l.file, txt, trans, lang, user)
}

func (l *TranslationLog) LangsCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state.langsCount()
}

func (l *TranslationLog) StringsCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state.stringsCount()
}

func (l *TranslationLog) UntranslatedCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state.untranslatedCount()
}

func (l *TranslationLog) LangInfos() []*LangInfo {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state.langInfos()
}

func (l *TranslationLog) recentEdits(max int) []Edit {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state.recentEdits(max)
}

func (l *TranslationLog) translators() []*Translator {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state.translators()
}

func (l *TranslationLog) updateStringsList(newStrings []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// for faster detection if string exists in newStrings, build a hash
	stringsHash := make(map[string]bool)
	for _, s := range newStrings {
		stringsHash[s] = true
	}

	var toUndelete []string
	for _, s := range newStrings {
		if strId, ok := l.state.stringMap[s]; ok {
			if l.state.isDeleted(strId) {
				toUndelete = append(toUndelete, s)
			}
		}
	}

	var toDelete []string
	for str, _ := range l.state.stringMap {
		if _, ok := stringsHash[str]; !ok {
			toDelete = append(toDelete, str)
		}
	}

	var toAdd []string
	for str, _ := range stringsHash {
		if _, ok := l.state.stringMap[str]; !ok {
			toAdd = append(toAdd, str)
		}
	}

	for _, str := range toUndelete {
		fmt.Printf("updateStringsList(): undelete %s\n", str)
		if err := l.state.undeleteString(l.file, str); err != nil {
			return err
		}
	}
	for _, str := range toDelete {
		fmt.Printf("updateStringsList(): delete %s\n", str)
		if err := l.state.deleteString(l.file, str); err != nil {
			return err
		}
	}
	for _, str := range toAdd {
		fmt.Printf("updateStringsList(): add %s\n", str)
		if err := l.state.writeNewStringRecord(l.file, str); err != nil {
			return err
		}
	}
	return nil
}
