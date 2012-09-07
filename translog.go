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
//   TODO: remember time?
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
	recNo   = 1
)

type TranslationRec struct {
	langId      int
	userId      int
	stringId    int
	translation string
}

type EncoderDecoderState struct {
	langCodeMap    map[string]int
	userNameMap    map[string]int
	stringMap      map[string]int
	deletedStrings map[int]bool
	translations   []TranslationRec
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

func encodeRecordData(rec []byte) *bytes.Buffer {
	var b bytes.Buffer
	panicIf(0 == len(rec), "0 == recLen")
	writeUvarintToBuf(&b, len(rec))
	b.Write(rec) // // ignore error, it's always nil
	return &b
}

func writeRecord(w io.Writer, rec []byte) error {
	b := encodeRecordData(rec)
	_, err := w.Write(b.Bytes())
	return err
}

func writeStringInternRecord(buf *bytes.Buffer, recId int, s string) {
	var b bytes.Buffer
	b.WriteByte(0)
	b.WriteByte(byte(recId))
	b.WriteString(s)
	// ignoring error because writing to bytes.Buffer never fails
	writeRecord(buf, b.Bytes())
}

func writeDeleteStringRecord(w io.Writer, strId int) error {
	var b bytes.Buffer
	b.WriteByte(0)
	b.WriteByte(strDelRec)
	writeUvarintToBuf(&b, strId)
	return writeRecord(w, b.Bytes())
}

func writeUndeleteStringRecord(w io.Writer, strId int) error {
	var b bytes.Buffer
	b.WriteByte(0)
	b.WriteByte(strUndelRec)
	writeUvarintToBuf(&b, strId)
	return writeRecord(w, b.Bytes())
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

func (s *EncoderDecoderState) addTranslationRec(langId, userId, stringId int, translation string) {
	t := &TranslationRec{langId, userId, stringId, translation}
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
	writeRecord(&recs, b.Bytes()) // cannot fail

	if _, err := w.Write(recs.Bytes()); err != nil {
		return err
	}
	s.addTranslationRec(langId, userId, stringId, trans)
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
		fmt.Printf("decodeStrRecord(): rec %d, %s -> %v in %s\n", recNo, str, id, tp)
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
		fmt.Printf("decodeStringDeleteRecord(): rec %d, %d\n", recNo, id)
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
		fmt.Printf("decodeStringUndeleteRecord(): rec %d, %d\n", recNo, id)
	}
	delete(s.deletedStrings, int(id))
	return nil
}

// rec is: varint(stringId) varint(userId) varint(langId) string
func (s *EncoderDecoderState) decodeNewTranslation(rec []byte) error {
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
	s.addTranslationRec(int(langId), int(userId), int(stringId), translation)
	if logging {
		fmt.Printf("decodeNewTranslation(): rec %d, %v, %v, %v, %s\n", recNo, langId, userId, stringId, translation)
	}
	return nil
}

func (s *EncoderDecoderState) decodeRecord(rec []byte) error {
	recNo++
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
		return s.decodeNewTranslation(rec)
	}
	return nil
}

// TODO: optimize by passing in a buffer to fill instead of allocating a new
// one every time?
func readRecord(r *ReaderByteReader) ([]byte, error) {
	n, err := readUVarintAsInt(r)
	if err != nil {
		if err == io.EOF {
			return nil, nil
		}
		log.Fatalf("readRecord(), err: %s", err.Error())
	}
	panicIf(0 == n, "recSize is 0")
	buf := make([]byte, n)
	n, err = r.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf, err
}

func (s *EncoderDecoderState) readExistingRecords(r *ReaderByteReader) error {
	for {
		// TODO: not sure what to do about an error here
		//lastValidOffset, _ := l.file.Seek(1, 0)
		// TODO: could explicitly see if this is EOF by comparing lastValidOffset
		// with file size. that would simplify error handling
		buf, err := readRecord(r)
		if err != nil {
			// TODO: do something on error
			panic("readExistingRecords")
		}
		if buf == nil {
			return nil
		}
		err = s.decodeRecord(buf)
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
	t.Translations = make([]string, 0)
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

func (l *TranslationLog) updateStringsList(newStrings []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// for faster detection if string exists in newStrings, build a hash
	stringsHash := make(map[string]bool)
	for _, s := range newStrings {
		stringsHash[s] = true
	}

	toUndelete := make([]string, 0)
	for _, s := range newStrings {
		if strId, ok := l.state.stringMap[s]; ok {
			if l.state.isDeleted(strId) {
				toUndelete = append(toUndelete, s)
			}
		}
	}

	toDelete := make([]string, 0)
	for str, _ := range l.state.stringMap {
		if _, ok := stringsHash[str]; !ok {
			toDelete = append(toDelete, str)
		}
	}

	toAdd := make([]string, 0)
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
