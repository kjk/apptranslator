package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
)

// Translation log is append-only file. It consists of blocks. Each block
// starts with uvarint-encoded length followed by block data of that length.
// This allows to detect and fix a corruption caused by not fully writing the
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

type TranslationLog struct {
	filePath       string
	file           *os.File
	langCodeMap    map[string]int
	userNameMap    map[string]int
	stringMap      map[string]int
	deletedStrings map[int]bool
	translations   []TranslationRec
}

var errorRecordMalformed = errors.New("Record malformed")

func (l *TranslationLog) writeRecord(rec []byte) error {
	var buf [32]byte
	n := binary.PutUvarint(buf[:], uint64(len(rec)))
	_, err := l.file.Write(buf[:n])
	if err != nil {
		return err
	}
	_, err = l.file.Write(rec)
	return err
}

func (l *TranslationLog) writeStringInternRecord(recId int, s string) error {
	var b bytes.Buffer
	b.WriteByte(0)
	b.WriteByte(byte(recId))
	b.WriteString(s)
	_, err := b.WriteTo(l.file)
	return err
}

// returns a unique integer 1..n for a given string
// if the id hasn't been seen yet
func (l *TranslationLog) uniquifyString(s string, dict map[string]int, recId int) (int, error) {
	if n, ok := dict[s]; ok {
		return n, nil
	}
	n := len(dict) + 1
	dict[s] = n
	err := l.writeStringInternRecord(recId, s)
	return n, err
}

func writeVarintToBuf(b *bytes.Buffer, i int) {
	var buf [32]byte
	n := binary.PutVarint(buf[:], int64(i))
	b.Write(buf[:n]) // ignore error, it's always nil
}

func (l *TranslationLog) addTranslationRec(langId, userId, stringId int, translation string) {
	t := &TranslationRec{langId, userId, stringId, translation}
	l.translations = append(l.translations, *t)
}

func (l *TranslationLog) writeNewTranslation(txt, trans, lang, user string) error {
	var b bytes.Buffer
	var userId, stringId int
	langId, err := l.uniquifyString(lang, l.langCodeMap, newLangIdRec)
	if err != nil {
		return err
	}
	writeVarintToBuf(&b, langId)
	userId, err = l.uniquifyString(user, l.userNameMap, newUserNameIdRec)
	if err != nil {
		return err
	}
	writeVarintToBuf(&b, userId)
	stringId, err = l.uniquifyString(txt, l.stringMap, newStringIdRec)
	if err != nil {
		return err
	}
	writeVarintToBuf(&b, stringId)
	b.Write([]byte(trans))
	if err = l.writeRecord(b.Bytes()); err != nil {
		return err
	}
	l.addTranslationRec(langId, userId, stringId, trans)
	return nil
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
	l.langCodeMap = make(map[string]int)
	l.userNameMap = make(map[string]int)
	l.stringMap = make(map[string]int)
	l.translations = make([]TranslationRec, 0)
	l.deletedStrings = make(map[int]bool)

	if existing {
		err = l.readExistingRecords()
		if nil != err {
			l.close()
			log.Fatalf("Failed to read log '%s'\n", path)
		}
	}
	return l, nil
}

func (l *TranslationLog) close() {
	l.file.Close()
	l.file = nil
}

// wrap os.File and provide ReadByte() so that we can use
// binary.ReadVarint() on it
// TODO: could just put it directly on TranslationLog, eliminating
// the need for ByteReaderForFile struct
type ByteReaderForFile struct {
	file *os.File
}

func (r *ByteReaderForFile) ReadByte() (byte, error) {
	var buf [1]byte
	_, err := r.file.Read(buf[0:1])
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

func decodeIdString(rec []byte) (int, string, error) {
	id, n := binary.Varint(rec)
	if n <= 0 || n == len(rec) {
		return 0, "", errorRecordMalformed
	}
	str := string(rec[n:])
	return int(id), str, nil
}

// rec is: varint(langId) string
func (l *TranslationLog) decodeNewLangIdRecord(rec []byte) error {
	if id, str, err := decodeIdString(rec); err != nil {
		return err
	} else {
		// TODO: verify doesn't already exist and id is consequitive
		l.langCodeMap[str] = id
	}
	return nil
}

// rec is: varint(userId) string
func (l *TranslationLog) decodeNewUserNameRecord(rec []byte) error {
	if id, str, err := decodeIdString(rec); err != nil {
		return err
	} else {
		// TODO: verify doesn't already exist and id is consequitive
		l.userNameMap[str] = id
	}
	return nil
}

// rec is: varint(stringId) string
func (l *TranslationLog) decodeNewStringRecord(rec []byte) error {
	if id, str, err := decodeIdString(rec); err != nil {
		return err
	} else {
		// TODO: verify doesn't already exist and id is consequitive
		l.stringMap[str] = id
	}
	return nil
}

// rec is: varint(stringId)
func (l *TranslationLog) decodeStringDeleteRecord(rec []byte) error {
	id, n := binary.Varint(rec)
	if n != len(rec) {
		return errorRecordMalformed
	}
	// TODO: make sure doesn't already exist
	l.deletedStrings[int(id)] = true
	return nil
}

// rec is: varint(stringId)
func (l *TranslationLog) decodeStringUndeleteRecord(rec []byte) error {
	id, n := binary.Varint(rec)
	if n != len(rec) {
		return errorRecordMalformed
	}
	// TODO: make sure exists
	delete(l.deletedStrings, int(id))
	return nil
}

// rec is: varint(stringId) varint(userId) varint(langId) string
func (l *TranslationLog) decodeNewTranslation(rec []byte) error {
	var stringId, userId, langId int64
	var n int

	stringId, n = binary.Varint(rec)
	if n <= 0 || n == len(rec) {
		return errorRecordMalformed
	}
	rec = rec[n:]

	userId, n = binary.Varint(rec)
	if n <= 0 || n == len(rec) {
		return errorRecordMalformed
	}
	rec = rec[n:]

	langId, n = binary.Varint(rec)
	if n <= 0 || n == len(rec) {
		return errorRecordMalformed
	}
	translation := string(rec[n:])
	l.addTranslationRec(int(langId), int(userId), int(stringId), translation)
	return nil
}

func (l *TranslationLog) decodeRecord(rec []byte) error {
	if 0 == len(rec) {
		return errorRecordMalformed
	}
	t := rec[0]
	switch t {
	case newLangIdRec:
		return l.decodeNewLangIdRecord(rec[1:])
	case newUserNameIdRec:
		return l.decodeNewUserNameRecord(rec[1:])
	case newStringIdRec:
		return l.decodeNewStringRecord(rec[1:])
	case strDelRec:
		return l.decodeStringDeleteRecord(rec[1:])
	case strUndelRec:
		return l.decodeStringUndeleteRecord(rec[1:])
	default:
		return l.decodeNewTranslation(rec)
	}
	return nil
}

func (l *TranslationLog) readExistingRecords() error {
	var n int
	r := &ByteReaderForFile{l.file}
	buf := make([]byte, 512)
	for {
		// TODO: not sure what to do about an error here
		//lastValidOffset, _ := l.file.Seek(1, 0)
		// TODO: could explicitly see if this is EOF by comparing lastValidOffset
		// with file size. that would simplify error handling
		tmp, err := binary.ReadUvarint(r)
		if err != nil {
			if err == io.EOF {
				// TODO: probably truncate if len != 0, as that might indicate
				// EOF in the middle of reading the varint
				return nil
			}
		}
		recSize := int(tmp)
		if recSize > cap(buf) {
			buf = make([]byte, recSize)
		}
		n, err = l.file.Read(buf[0:recSize])
		if err != nil || n != recSize {
			// TODO: truncate to lastValidOffset and return nil
			return err
		}
		err = l.decodeRecord(buf[0:recSize])
		if err != nil {
			return err
		}
	}
	panic("not reached")
	return nil
}

