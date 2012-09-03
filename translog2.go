package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
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
//   byte(0) byte(1) varint(langId) string
//
// - record interning a new user name string, format:
//
//   byte(0) byte(2) varint(userId) string
//
// - record interning a new string for translation, format:
//
//   byte(0) byte(3) varint(stringId) string
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
//   varint(stringId) varint(userId) varint(langId) string
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

type TranslationLog struct {
	state    *EncoderDecoderState
	filePath string
	file     *os.File

	// protects writing translations
	mu         sync.Mutex
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

func writeStringInternRecord(buf *bytes.Buffer, recId, n int, s string) {
	var b bytes.Buffer
	b.WriteByte(0)
	b.WriteByte(byte(recId))
	writeUvarintToBuf(&b, n)
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
func uniquifyString(buf *bytes.Buffer, s string, dict map[string]int, recId int) int {
	if n, ok := dict[s]; ok {
		return n
	}
	n := len(dict) + 1
	dict[s] = n
	writeStringInternRecord(buf, recId, n, s)
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
		if _, ok := s.deletedStrings[strId]; !ok {
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
	langId := uniquifyString(&recs, lang, s.langCodeMap, newLangIdRec)
	userId := uniquifyString(&recs, user, s.userNameMap, newUserNameIdRec)
	stringId := uniquifyString(&recs, txt, s.stringMap, newStringIdRec)

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

func decodeIdString(rec []byte) (int, string, error) {
	id, n := binary.Uvarint(rec)
	panicIf(n <= 0 || n == len(rec), "decodeIdString")
	str := string(rec[n:])
	return int(id), str, nil
}

// rec is: varint(langId) string
func decodeStrIdRecord(rec []byte, dict map[string]int) error {
	if id, str, err := decodeIdString(rec); err != nil {
		return err
	} else {
		//fmt.Printf("decodeStrIdRecord(): %s -> %d\n", str, id)
		if _, exists := dict[str]; exists {
			log.Fatalf("decodeStrIdRecord(): '%s' already exists in dict\n", str)
		}
		dict[str] = id
	}
	return nil
}

// rec is: varint(stringId)
func (s *EncoderDecoderState) decodeStringDeleteRecord(rec []byte) error {
	id, n := binary.Uvarint(rec)
	panicIf(n != len(rec), "decodeStringDeleteRecord")
	if _, exists := s.deletedStrings[int(id)]; exists {
		log.Fatalf("decodeStrIdRecord(): '%d' already exists in deletedString\n", id)
	}
	s.deletedStrings[int(id)] = true
	return nil
}

// rec is: varint(stringId)
func (s *EncoderDecoderState) decodeStringUndeleteRecord(rec []byte) error {
	id, n := binary.Uvarint(rec)
	panicIf(n != len(rec), "decodeStringUndeleteRecord")
	if _, exists := s.deletedStrings[int(id)]; !exists {
		log.Fatalf("decodeStringUndeleteRecord(): '%d' doesn't exists in deletedStrings\n", id)
	}

	delete(s.deletedStrings, int(id))
	return nil
}

// rec is: varint(stringId) varint(userId) varint(langId) string
func (s *EncoderDecoderState) decodeNewTranslation(rec []byte) error {
	var stringId, userId, langId uint64
	var n int

	stringId, n = binary.Uvarint(rec)
	panicIf(n == 0 || n == len(rec), "decodeNewTranslation")

	rec = rec[n:]

	userId, n = binary.Uvarint(rec)
	panicIf(n == len(rec), "decodeNewTranslation2")
	rec = rec[n:]

	langId, n = binary.Uvarint(rec)
	panicIf(n <= 0 || n == len(rec), "decodeNewTranslation3")
	translation := string(rec[n:])
	s.addTranslationRec(int(langId), int(userId), int(stringId), translation)
	return nil
}

var recNo = 1

func (s *EncoderDecoderState) decodeRecord(rec []byte) error {
	fmt.Printf("decodeRecord %d\n", recNo)
	recNo++
	panicIf(len(rec) < 2, "decodeRecord(), len(rec) < 2")
	if 0 == rec[0] {
		t := rec[1]
		switch t {
		case newLangIdRec:
			return decodeStrIdRecord(rec[2:], s.langCodeMap)
		case newUserNameIdRec:
			return decodeStrIdRecord(rec[2:], s.userNameMap)
		case newStringIdRec:
			return decodeStrIdRecord(rec[2:], s.stringMap)
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
