package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
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

func writeVarintToBuf(b *bytes.Buffer, i int) {
	var buf [32]byte
	n := binary.PutVarint(buf[:], int64(i))
	b.Write(buf[:n]) // ignore error, it's always nil
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
	writeVarintToBuf(&b, len(rec))
	b.Write(rec) // // ignore error, it's always nil
	return &b
}

func writeRecord(w io.Writer, rec []byte) error {
	b := encodeRecordData(rec)
	data := b.Bytes()
	_, err := w.Write(data)
	return err
}

func writeStringInternRecord(w io.Writer, recId, n int, s string) error {
	var b bytes.Buffer
	//fmt.Printf("writeStringInternRecord %d %d %s\n", recId, n, s)
	b.WriteByte(0)
	b.WriteByte(byte(recId))
	writeVarintToBuf(&b, n)
	b.WriteString(s)
	return writeRecord(w, b.Bytes())
}

// returns a unique integer 1..n for a given string
// if the id hasn't been seen yet
func uniquifyString(w io.Writer, s string, dict map[string]int, recId int) (int, error) {
	if n, ok := dict[s]; ok {
		return n, nil
	}
	n := len(dict) + 1
	dict[s] = n
	err := writeStringInternRecord(w, recId, n, s)
	return n, err
}

func (s *EncoderDecoderState) addTranslationRec(langId, userId, stringId int, translation string) {
	t := &TranslationRec{langId, userId, stringId, translation}
	s.translations = append(s.translations, *t)
}

func (s *EncoderDecoderState) writeNewTranslation(w io.Writer, txt, trans, lang, user string) error {
	var b bytes.Buffer
	var userId, stringId int
	//fmt.Printf("writeNewTranslation\n")
	langId, err := uniquifyString(w, lang, s.langCodeMap, newLangIdRec)
	if err != nil {
		return err
	}
	writeVarintToBuf(&b, langId)
	userId, err = uniquifyString(w, user, s.userNameMap, newUserNameIdRec)
	if err != nil {
		return err
	}
	writeVarintToBuf(&b, userId)
	stringId, err = uniquifyString(w, txt, s.stringMap, newStringIdRec)
	if err != nil {
		return err
	}
	writeVarintToBuf(&b, stringId)
	b.Write([]byte(trans))
	if err = writeRecord(w, b.Bytes()); err != nil {
		return err
	}
	s.addTranslationRec(langId, userId, stringId, trans)
	return nil
}

func NewEncoderDecoderState() *EncoderDecoderState {
	s := &EncoderDecoderState{}
	s.langCodeMap = make(map[string]int)
	s.userNameMap = make(map[string]int)
	s.stringMap = make(map[string]int)
	s.translations = make([]TranslationRec, 0)
	s.deletedStrings = make(map[int]bool)
	return s
}

func NewTranslationLog(path string) (*TranslationLog, error) {
	existing := true
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			file, err = os.Create(path)
			if err != nil {
				return nil, err
			}
			existing = false
		}
	}
	l := &TranslationLog{filePath: path, file: file}
	l.state = NewEncoderDecoderState()
	if existing {
		r := &ReaderByteReader{file}
		err = l.state.readExistingRecords(r)
		if nil != err {
			l.close()
			log.Fatalf("Failed to read log '%s', err: %s\n", path, err.Error())
		}
	}
	return l, nil
}

func (l *TranslationLog) close() {
	l.file.Close()
	l.file = nil
}

func decodeIdString(rec []byte) (int, string, error) {
	id, n := binary.Varint(rec)
	panicIf(n <= 0 || n == len(rec), "decdeIdString")
	str := string(rec[n:])
	return int(id), str, nil
}

// rec is: varint(langId) string
func decodeStrIdRecord(rec []byte, dict map[string]int) error {
	if id, str, err := decodeIdString(rec); err != nil {
		return err
	} else {
		// TODO: verify doesn't already exist and id is consequitive
		dict[str] = id
	}
	return nil
}

// rec is: varint(stringId)
func (s *EncoderDecoderState) decodeStringDeleteRecord(rec []byte) error {
	id, n := binary.Varint(rec)
	panicIf(n != len(rec), "decodeStringDeleteRecord")
	// TODO: make sure doesn't already exist
	s.deletedStrings[int(id)] = true
	return nil
}

// rec is: varint(stringId)
func (s *EncoderDecoderState) decodeStringUndeleteRecord(rec []byte) error {
	id, n := binary.Varint(rec)
	panicIf(n != len(rec), "decodeStringUndeleteRecord")
	// TODO: make sure exists
	delete(s.deletedStrings, int(id))
	return nil
}

// rec is: varint(stringId) varint(userId) varint(langId) string
func (s *EncoderDecoderState) decodeNewTranslation(rec []byte) error {
	var stringId, userId, langId int64
	var n int

	stringId, n = binary.Varint(rec)
	panicIf(n <= 0 || n == len(rec), "decodeNewTranslation")

	rec = rec[n:]

	userId, n = binary.Varint(rec)
	panicIf(n <= 0 || n == len(rec), "decodeNewTranslation2")
	rec = rec[n:]

	langId, n = binary.Varint(rec)
	panicIf(n <= 0 || n == len(rec), "decodeNewTranslation3")
	translation := string(rec[n:])
	s.addTranslationRec(int(langId), int(userId), int(stringId), translation)
	return nil
}

var recNo int

func (s *EncoderDecoderState) decodeRecord(rec []byte) error {
	fmt.Printf("decodeRecord %d\n", recNo)
	recNo++
	panicIf(0 == len(rec), "decodeRecord")
	t := rec[0]
	switch t {
	case newLangIdRec:
		return decodeStrIdRecord(rec[1:], s.langCodeMap)
	case newUserNameIdRec:
		return decodeStrIdRecord(rec[1:], s.userNameMap)
	case newStringIdRec:
		return decodeStrIdRecord(rec[1:], s.stringMap)
	case strDelRec:
		return s.decodeStringDeleteRecord(rec[1:])
	case strUndelRec:
		return s.decodeStringUndeleteRecord(rec[1:])
	default:
		return s.decodeNewTranslation(rec)
	}
	return nil
}

// TODO: optimize by passing in a buffer to fill instead of allocating a new
// one every time?
func readRecord(r *ReaderByteReader) ([]byte, error) {
	n, err := readUVarintAsInt(r)
	if err != nil {
		log.Fatalf("readRecord(), err: %s", err.Error())
	}
	panicIf(0 == n, "recSize is 0")
	buf := make([]byte, 0, n)
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
		err = s.decodeRecord(buf)
		if err != nil {
			return err
		}
	}
	panic("not reached")
	return nil
}
