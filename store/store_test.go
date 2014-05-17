// This code is under BSD license. See license-bsd.txt
package store

import (
	"bytes"
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

func (ts *TestState) getWriter() io.Writer {
	if nil != ts.l {
		return ts.l.file
	}
	return ts.w
}

func (ts *TestState) getStateWriter() (*EncoderDecoderState, io.Writer) {
	return ts.getEncoderDecoderState(), ts.getWriter()
}

func (ts *TestState) writeNewTranslation(s1, s2, s3, s4 string) {
	s, w := ts.getStateWriter()
	if err := s.writeNewTranslation(w, s1, s2, s3, s4); err != nil {
		ts.t.Fatal(err)
	}
}

func (ts *TestState) deleteString(str string) {
	s, w := ts.getStateWriter()
	if err := s.deleteString(w, str); err != nil {
		ts.t.Fatal(err)
	}
}

func (ts *TestState) undeleteString(str string) {
	s, w := ts.getStateWriter()
	if err := s.undeleteString(w, str); err != nil {
		ts.t.Fatal(err)
	}
}

func (ts *TestState) UpdateStringsList(s []string) ([]string, []string, []string) {
	added, deleted, undeleted, err := ts.l.UpdateStringsList(s)
	if err != nil {
		ts.t.Fatal(err)
	}
	return added, deleted, undeleted
}

func (ts *TestState) ensureLangCount(expected int) {
	if s := ts.getEncoderDecoderState(); len(s.langCodeMap) != expected {
		ts.t.Fatalf("len(s.langCodeMap)=%d, expected: %d", len(s.langCodeMap), expected)
	}
}

func (ts *TestState) ensureUserCount(expected int) {
	if s := ts.getEncoderDecoderState(); len(s.userNameMap) != expected {
		ts.t.Fatalf("len(s.userNameMap)=%d, expected: %d", len(s.userNameMap), expected)
	}
}

func (ts *TestState) ensureStringsCount(expected int) {
	if s := ts.getEncoderDecoderState(); len(s.stringMap) != expected {
		ts.t.Fatalf("len(s.stringMap)=%d, expected: %d", len(s.stringMap), expected)
	}
}

func (ts *TestState) ensureUndeletedStringsCount(expected int) {
	s := ts.getEncoderDecoderState()
	if n := s.stringsCount(); n != expected {
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
	if s := ts.getEncoderDecoderState(); s.langCodeMap[name] != expected {
		ts.t.Fatalf("s.langCodeMap['%s']=%d, expected: %d", name, s.langCodeMap[name], expected)
	}
}

func (ts *TestState) ensureUserCode(name string, expected int) {
	if s := ts.getEncoderDecoderState(); s.userNameMap[name] != expected {
		ts.t.Fatalf("s.userNameMap['%s']=%d, expected: %d", name, s.userNameMap[name], expected)
	}
}

func (ts *TestState) ensureStringCode(name string, expected int) {
	if s := ts.getEncoderDecoderState(); s.stringMap[name] != expected {
		ts.t.Fatalf("s.stringMap['%s']=%d, expected: %d", name, s.stringMap[name], expected)
	}
}

func (ts *TestState) ensureDeletedCount(expected int) {
	if s := ts.getEncoderDecoderState(); len(s.deletedStrings) != expected {
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

func (ts *TestState) ensureStringsAre(strs []string) {
	ts.ensureUndeletedStringsCount(len(strs))
	s := ts.getEncoderDecoderState()
	for _, str := range strs {
		if _, exist := s.stringMap[str]; !exist {
			ts.t.Fatalf("'%s' doesn't exist in s.stringMap", str)
		}
	}

}

func NewTranslationLogTestState(t *testing.T, path string) *TestState {
	if l, err := NewTranslationLog(path); err != nil {
		t.Fatal("Failed to create new trans_test.dat")
	} else {
		return &TestState{t, l, nil, nil}
	}
	return nil
}

func TestTransLog(t *testing.T) {
	var buf bytes.Buffer
	var err error

	s := NewEncoderDecoderState()
	ts := &TestState{t, nil, s, &buf}
	ts.ensureStateEmpty()

	ts.writeNewTranslation("foo", "foo-us", "us", "user1")
	ts.ensureStateAfter1()

	ts.writeNewTranslation("foo", "foo-pl", "pl", "user1")
	ts.ensureStateAfter2()

	ts.writeNewTranslation("bar", "bar-pl", "pl", "user1")
	ts.ensureStateAfter3()

	ts.deleteString("bar")
	ts.ensureDeletedCount(1)
	ts.ensureUndeletedStringsCount(1)

	ts.undeleteString("bar")
	ts.ensureDeletedCount(0)
	ts.ensureUndeletedStringsCount(2)

	// test reading from scratch
	s = NewEncoderDecoderState()
	if err = s.readExistingRecords(&ReaderByteReader{&buf}); err != nil {
		t.Fatal(err)
	}
	ts.ensureStateAfter3()
}

// test appending to existing translation log works
func TestTransLog2(t *testing.T) {
	path := "transtest.dat"
	os.Remove(path) // just in case

	ts := NewTranslationLogTestState(t, path)
	ts.ensureStateEmpty()

	ts.writeNewTranslation("foo", "foo-us", "us", "user1")
	ts.ensureStateAfter1()
	ts.l.close()

	ts = NewTranslationLogTestState(t, path)
	ts.ensureStateAfter1()

	ts.writeNewTranslation("foo", "foo-pl", "pl", "user1")
	ts.ensureStateAfter2()
	ts.l.close()

	ts = NewTranslationLogTestState(t, path)
	ts.ensureStateAfter2()

	added, deleted, undeleted := ts.UpdateStringsList([]string{"foo", "bar", "go"})
	if len(added) != 2 {
		t.Fatalf("len(added) != 2")
	}
	if len(deleted) != 0 {
		t.Fatalf("len(deleted) != 0")
	}
	if len(undeleted) != 0 {
		t.Fatalf("len(undeleted) != 0")
	}
	ts.ensureStringsAre([]string{"foo", "bar", "go"})

	_, deleted, _ = ts.UpdateStringsList([]string{"foo", "bar"})
	if len(deleted) != 1 {
		t.Fatalf("len(deleted) != 1")
	}

	_, _, undeleted = ts.UpdateStringsList([]string{"foo", "bar", "go"})
	if len(undeleted) != 1 {
		t.Fatalf("len(undeleted) != 1")
	}

	ts.l.close()
}
