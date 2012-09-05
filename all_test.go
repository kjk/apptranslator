package main

import (
	"bytes"
	"fmt"
	"testing"
	"os"
)

func errIf(t *testing.T, cond bool, msg string) {
	if cond {
		t.Error(msg)
	}
}

func ensureLangCount(t *testing.T, s *EncoderDecoderState, expected int) {
	if len(s.langCodeMap) != expected {
		t.Fatalf("len(s.langCodeMap)=%d, expected: %d", len(s.langCodeMap), expected)
	}
}

func ensureUserCount(t *testing.T, s *EncoderDecoderState, expected int) {
	if len(s.userNameMap) != expected {
		t.Fatalf("len(s.userNameMap)=%d, expected: %d", len(s.userNameMap), expected)
	}
}

func ensureStringsCount(t *testing.T, s *EncoderDecoderState, expected int) {
	if len(s.stringMap) != expected {
		t.Fatalf("len(s.stringMap)=%d, expected: %d", len(s.stringMap), expected)
	}
}

func ensureTranslationsCount(t *testing.T, s *EncoderDecoderState, expected int) {
	if len(s.translations) != expected {
		t.Fatalf("len(s.translations)=%d, expected: %d", len(s.translations), expected)
	}
}

func ensureLangCode(t *testing.T, s *EncoderDecoderState, name string, expected int) {
	if s.langCodeMap[name] != expected {
		t.Fatalf("s.langCodeMap['%s']=%d, expected: %d", s.langCodeMap[name], expected)
	}
}

func ensureUserCode(t *testing.T, s *EncoderDecoderState, name string, expected int) {
	if s.userNameMap[name] != expected {
		t.Fatalf("s.userNameMap['%s']=%d, expected: %d", s.userNameMap[name], expected)
	}
}

func ensureStringCode(t *testing.T, s *EncoderDecoderState, name string, expected int) {
	if s.stringMap[name] != expected {
		t.Fatalf("s.stringMap['%s']=%d, expected: %d", s.stringMap[name], expected)
	}
}

func ensureStateEmpty(t *testing.T, s *EncoderDecoderState) {
	ensureLangCount(t, s, 0)
	ensureUserCount(t, s, 0)
	ensureStringsCount(t, s, 0)
}

func ensureStateAfter1(t *testing.T, s *EncoderDecoderState) {
	ensureLangCount(t, s, 1)
	ensureLangCode(t, s, "us", 1)
	ensureUserCount(t, s, 1)
	ensureUserCode(t, s, "user1", 1)
	ensureStringsCount(t, s, 1)
	ensureTranslationsCount(t, s, 1)
}

func ensureStateAfter2(t *testing.T, s *EncoderDecoderState) {
	ensureLangCount(t, s, 2)
	ensureLangCode(t, s, "pl", 2)
	ensureUserCount(t, s, 1)
	ensureTranslationsCount(t, s, 2)
}

func ensureStateAfter3(t *testing.T, s *EncoderDecoderState) {
	ensureLangCount(t, s, 2)
	ensureUserCount(t, s, 1)
	ensureLangCode(t, s, "us", 1)
	ensureUserCode(t, s, "user1", 1)
	ensureLangCode(t, s, "pl", 2)
	ensureTranslationsCount(t, s, 3)	
}

func NewTranslationLogEnsure(t *testing.T, path string) *TranslationLog {
	l, err := NewTranslationLog(path)
	if err != nil {
		t.Fatal("Failed to create new trans_test.dat")
	}
	return l
}

// test appending to existing translation log works
func TestTransLog2(t *testing.T) {
	path := "transtest.dat"
	os.Remove(path) // just in case

	l := NewTranslationLogEnsure(t, path)
	ensureStateEmpty(t, l.state)
	err := l.writeNewTranslation("foo", "foo-us", "us", "user1")
	if err != nil {
		t.Fatal(err)
	}
	ensureStateAfter1(t, l.state)
	l.close()

	l = NewTranslationLogEnsure(t, path)
	ensureStateAfter1(t, l.state)

	err = l.writeNewTranslation("foo", "foo-pl", "pl", "user1")
	if err != nil {
		t.Fatal(err)
	}
	ensureStateAfter2(t, l.state)
	l.close()

	l = NewTranslationLogEnsure(t, path)
	ensureStateAfter2(t, l.state)
	l.close()
}

func TestTransLog(t *testing.T) {
	var buf bytes.Buffer

	s := NewEncoderDecoderState()
	ensureStateEmpty(t, s)

	err := s.writeNewTranslation(&buf, "foo", "foo-us", "us", "user1")
	if err != nil {
		t.Fatal(err)
	}
	ensureStateAfter1(t, s)

	err = s.writeNewTranslation(&buf, "foo", "foo-pl", "pl", "user1")
	if err != nil {
		t.Fatal(err)
	}
	ensureStateAfter2(t, s)

	err = s.writeNewTranslation(&buf, "bar", "bar-pl", "pl", "user1")
	if err != nil {
		t.Fatal(err)
	}
	ensureStateAfter3(t, s)

	err = s.deleteString(&buf, "bar")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.deletedStrings) != 1 {
		t.Fatal("len(s.deletedStrings)=", len(s.deletedStrings), ", expected: 1")
	}

	err = s.undeleteString(&buf, "bar")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.deletedStrings) != 0 {
		t.Fatal("len(s.deletedStrings)=", len(s.deletedStrings), ", expected: 0")
	}

	fmt.Printf("buf.Bytes()=%v\n", buf.Bytes())

	// test reading from scratch
	s = NewEncoderDecoderState()
	err = s.readExistingRecords(&ReaderByteReader{&buf})
	if err != nil {
		t.Fatal(err)
	}
	ensureStateAfter3(t, s)
}
