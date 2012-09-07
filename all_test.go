package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"
)

type TestState struct {
	t *testing.T
	// ether l is set or both s and w
	l *TranslationLog

	s *EncoderDecoderState
	w io.Writer
}

func (ts *TestState) getEncoderDecoderState() *EncoderDecoderState {
	if nil != ts.s {
		return ts.s
	}
	return ts.l.state
}

func (ts *TestState) ensureLangCount(expected int) {
	s := ts.getEncoderDecoderState()
	if len(s.langCodeMap) != expected {
		ts.t.Fatalf("len(s.langCodeMap)=%d, expected: %d", len(s.langCodeMap), expected)
	}
}

func (ts *TestState) ensureUserCount(expected int) {
	s := ts.getEncoderDecoderState()
	if len(s.userNameMap) != expected {
		ts.t.Fatalf("len(s.userNameMap)=%d, expected: %d", len(s.userNameMap), expected)
	}
}

func (ts *TestState) ensureStringsCount(expected int) {
	s := ts.getEncoderDecoderState()
	if len(s.stringMap) != expected {
		ts.t.Fatalf("len(s.stringMap)=%d, expected: %d", len(s.stringMap), expected)
	}
}

func (ts *TestState) ensureUndeletedStringsCount(expected int) {
	s := ts.getEncoderDecoderState()
	n := s.stringsCount()
	if n != expected {
		ts.t.Fatalf("len(s.stringsCount())=%d, expected: %d", s.stringsCount(), expected)
	}
}

func (ts *TestState) ensureTranslationsCount(expected int) {
	s := ts.getEncoderDecoderState()
	if len(s.translations) != expected {
		ts.t.Fatalf("len(s.translations)=%d, expected: %d", len(s.translations), expected)
	}
}

func (ts *TestState) ensureLangCode(name string, expected int) {
	s := ts.getEncoderDecoderState()
	if s.langCodeMap[name] != expected {
		ts.t.Fatalf("s.langCodeMap['%s']=%d, expected: %d", s.langCodeMap[name], expected)
	}
}

func (ts *TestState) ensureUserCode(name string, expected int) {
	s := ts.getEncoderDecoderState()
	if s.userNameMap[name] != expected {
		ts.t.Fatalf("s.userNameMap['%s']=%d, expected: %d", s.userNameMap[name], expected)
	}
}

func (ts *TestState) ensureStringCode(name string, expected int) {
	s := ts.getEncoderDecoderState()
	if s.stringMap[name] != expected {
		ts.t.Fatalf("s.stringMap['%s']=%d, expected: %d", s.stringMap[name], expected)
	}
}

func (ts *TestState) ensureDeletedCount(expected int) {
	s := ts.getEncoderDecoderState()
	if len(s.deletedStrings) != expected {
		ts.t.Fatalf("len(s.deletedStrings)=%d, expected: %d", len(s.deletedStrings), expected)
	}
}

func (ts *TestState) ensureStateEmpty() {
	ts.ensureLangCount(0)
	ts.ensureUserCount(0)
	ts.ensureStringsCount(0)
}

func (ts *TestState) ensureStateAfter1() {
	ts.ensureLangCount(1)
	ts.ensureLangCode("us", 1)
	ts.ensureUserCount(1)
	ts.ensureUserCode("user1", 1)
	ts.ensureStringsCount(1)
	ts.ensureTranslationsCount(1)
}

func (ts *TestState) ensureStateAfter2() {
	ts.ensureLangCount(2)
	ts.ensureLangCode("pl", 2)
	ts.ensureUserCount(1)
	ts.ensureTranslationsCount(2)
}

func (ts *TestState) ensureStateAfter3() {
	ts.ensureLangCount(2)
	ts.ensureUserCount(1)
	ts.ensureLangCode("us", 1)
	ts.ensureUserCode("user1", 1)
	ts.ensureLangCode("pl", 2)
	ts.ensureTranslationsCount(3)
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
	ts := &TestState{t, l, nil, nil}

	ts.ensureStateEmpty()

	err := l.writeNewTranslation("foo", "foo-us", "us", "user1")
	if err != nil {
		t.Fatal(err)
	}
	ts.ensureStateAfter1()
	l.close()

	l = NewTranslationLogEnsure(t, path)
	ts = &TestState{t, l, nil, nil}
	ts.ensureStateAfter1()

	err = l.writeNewTranslation("foo", "foo-pl", "pl", "user1")
	if err != nil {
		t.Fatal(err)
	}
	ts.ensureStateAfter2()
	l.close()

	l = NewTranslationLogEnsure(t, path)
	ts.ensureStateAfter2()
	l.close()
}

func writeNewTranslationEnsure(t *testing.T, s *EncoderDecoderState, w io.Writer, s1, s2, s3, s4 string) {
	err := s.writeNewTranslation(w, s1, s2, s3, s4)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTransLog(t *testing.T) {
	var buf bytes.Buffer
	var err error

	s := NewEncoderDecoderState()
	ts := &TestState{t, nil, s, &buf}
	ts.ensureStateEmpty()

	writeNewTranslationEnsure(t, s, &buf, "foo", "foo-us", "us", "user1")
	ts.ensureStateAfter1()

	writeNewTranslationEnsure(t, s, &buf, "foo", "foo-pl", "pl", "user1")
	ts.ensureStateAfter2()

	writeNewTranslationEnsure(t, s, &buf, "bar", "bar-pl", "pl", "user1")
	ts.ensureStateAfter3()

	err = s.deleteString(&buf, "bar")
	if err != nil {
		t.Fatal(err)
	}
	ts.ensureDeletedCount(1)
	ts.ensureUndeletedStringsCount(1)

	err = s.undeleteString(&buf, "bar")
	if err != nil {
		t.Fatal(err)
	}
	ts.ensureDeletedCount(0)
	ts.ensureUndeletedStringsCount(2)

	//newStrings := []string{"foo", "bar", "go"}
	//ts.updateStringsListEnsure(newStrings)

	fmt.Printf("buf.Bytes()=%v\n", buf.Bytes())

	// test reading from scratch
	s = NewEncoderDecoderState()
	err = s.readExistingRecords(&ReaderByteReader{&buf})
	if err != nil {
		t.Fatal(err)
	}
	ts.ensureStateAfter3()
}
