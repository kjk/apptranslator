package main

import (
	"encoding/binary"
	"io"
	"os"
)

// Translation log is append-only file. It consists of blocks. Each block
// starts with varint-encoded length followed by block data of that length.
// This allows to detect and fix a corruption caused by not fully writing the
// last record (e.g. because of power failure in the middle of the write).
//
// In order to minimize the size of the file, we intern some strings by assigning
// them unique, monotonically increasing integers, e.g. we identify languages
// by their code names like "en", "pl" etc. Instead of repeating 
//
// We have several types of records:
// - record interning a new language string
// - record interning a new user name string
// - record interning a new string for translation
// - record denoting a given string has been "deleted" i.e. the app no longer
//   needs the translation of that string. We don't delete past translations
//   of strings
// - record denoting a given string has been "undeleted", which is a faster
//   way to restore translations that to re-add them
// - record defining a single translation

type TranslationLog struct {
	filePath    string
	file        *os.File
	langCodeMap map[string]int
	userNameMap map[string]int
	stringMap   map[string]int
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
	if existing {
		err = l.readExistingRecords()
		if nil != err {
			l.close()
			return nil, err
		}
	}
	return l, nil
}

func (l *TranslationLog) close() {
	l.file.Close()
	l.file = nil
}

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

func (l *TranslationLog) readExistingRecords() error {
	var n int
	r := &ByteReaderForFile{l.file}
	buf := make([]byte, 512)
	for {
		// TODO: not sure what to do about an error here
		//lastValidOffset, _ := l.file.Seek(1, 0)
		// TODO: could explicitly see if this is EOF by comparing lastValidOffset
		// with file size. that would simplify error handling
		len, err := binary.ReadUvarint(r)
		if err != nil {
			if err == io.EOF {
				// TODO: probably truncate if len != 0, as that might indicate
				// EOF in the middle of reading the varint
				return nil
			}
		}
		if int(len) > cap(buf) {
			buf = make([]byte, len)
		}
		n, err = l.file.Read(buf[0:len])
		if err != nil || n != int(len) {
			// TODO: truncate to lastValidOffset and return nil
			return err
		}
	}
	panic("not reached")
	return nil
}

func (l *TranslationLog) write(langCode, s, trans string) error {
	// TODO: write me
	return nil
}
