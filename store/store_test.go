// This code is under BSD license. See license-bsd.txt
package store

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

type TestState struct {
	t *testing.T
	// ether l is set or both s and w
	s *StoreBinary
}

func (ts *TestState) writeNewTranslation(s1, s2, s3, s4 string) {
	if err := ts.s.WriteNewTranslation(s1, s2, s3, s4); err != nil {
		ts.t.Fatal(err)
	}
}

func (ts *TestState) DuplicateTranslation(origStr, newStr string) {
	if err := ts.s.DuplicateTranslation(origStr, newStr); err != nil {
		ts.t.Fatal(err)
	}
}

func (ts *TestState) deleteString(str string) {
	if err := ts.s.deleteString(ts.s.w, str); err != nil {
		ts.t.Fatal(err)
	}
}

func (ts *TestState) undeleteString(str string) {
	if err := ts.s.undeleteString(ts.s.w, str); err != nil {
		ts.t.Fatal(err)
	}
}

func (ts *TestState) UpdateStringsList(s []string) ([]string, []string, []string) {
	added, deleted, undeleted, err := ts.s.UpdateStringsList(s)
	if err != nil {
		ts.t.Fatal(err)
	}
	return added, deleted, undeleted
}

func (ts *TestState) ensureLangCount(expected int) {
	if s := ts.s; len(s.langCodeMap) != expected {
		ts.t.Fatalf("len(s.langCodeMap)=%d, expected: %d", len(s.langCodeMap), expected)
	}
}

func (ts *TestState) ensureUserCount(expected int) {
	if s := ts.s; len(s.userNameMap) != expected {
		ts.t.Fatalf("len(s.userNameMap)=%d, expected: %d", len(s.userNameMap), expected)
	}
}

func (ts *TestState) ensureStringsCount(expected int) {
	if s := ts.s; len(s.stringMap) != expected {
		ts.t.Fatalf("len(s.stringMap)=%d, expected: %d", len(s.stringMap), expected)
	}
}

func (ts *TestState) ensureUndeletedStringsCount(expected int) {
	s := ts.s
	if n := s.stringsCount(); n != expected {
		msg := fmt.Sprintf("len(s.stringsCount())=%d, expected: %d", s.stringsCount(), expected)
		panic(msg)
		ts.t.Fatal(msg)
	}
}

func (ts *TestState) ensureTranslationsCount(expected int) {
	s := ts.s
	if len(s.translations) != expected {
		ts.t.Fatalf("len(s.translations)=%d, expected: %d", len(s.translations), expected)
	}
}

func (ts *TestState) ensureLangCode(name string, expected int) {
	if s := ts.s; s.langCodeMap[name] != expected {
		ts.t.Fatalf("s.langCodeMap['%s']=%d, expected: %d", name, s.langCodeMap[name], expected)
	}
}

func (ts *TestState) ensureUserCode(name string, expected int) {
	if s := ts.s; s.userNameMap[name] != expected {
		ts.t.Fatalf("s.userNameMap['%s']=%d, expected: %d", name, s.userNameMap[name], expected)
	}
}

func (ts *TestState) ensureStringCode(name string, expected int) {
	if s := ts.s; s.stringMap[name] != expected {
		ts.t.Fatalf("s.stringMap['%s']=%d, expected: %d", name, s.stringMap[name], expected)
	}
}

func (ts *TestState) ensureDeletedCount(expected int) {
	if s := ts.s; len(s.deletedStrings) != expected {
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

func (ts *TestState) ensureStateAfter4() {
	ts.ensureLangCount(2)
	ts.ensureUserCount(1)
	ts.ensureLangCode("us", 1)
	ts.ensureUserCode("user1", 1)
	ts.ensureLangCode("pl", 2)
	ts.ensureTranslationsCount(5)
}

func (ts *TestState) ensureStringsAre(strs []string) {
	ts.ensureUndeletedStringsCount(len(strs))
	s := ts.s
	for _, str := range strs {
		if _, exist := s.stringMap[str]; !exist {
			ts.t.Fatalf("'%s' doesn't exist in s.stringMap", str)
		}
	}
}

func NewTranslationLogTestState(t *testing.T, path string) *TestState {
	if s, err := NewStoreBinary(path); err != nil {
		t.Fatal("Failed to create new trans_test.dat")
	} else {
		return &TestState{t, s}
	}
	return nil
}

func TestTransLog(t *testing.T) {
	var buf bytes.Buffer

	s, err := NewStoreBinaryWithWriter(&buf)
	ts := &TestState{t, s}
	ts.ensureStateEmpty()

	ts.writeNewTranslation("foo", "foo-us", "us", "user1")
	ts.ensureStateAfter1()

	ts.writeNewTranslation("foo", "foo-pl", "pl", "user1")
	ts.ensureStateAfter2()

	ts.writeNewTranslation("bar", "bar-pl", "pl", "user1")
	ts.ensureStateAfter3()

	ts.DuplicateTranslation("foo", "foo2")
	ts.ensureStateAfter4()

	ts.deleteString("bar")
	ts.ensureDeletedCount(1)
	ts.ensureUndeletedStringsCount(2)

	ts.undeleteString("bar")
	ts.ensureDeletedCount(0)
	ts.ensureUndeletedStringsCount(3)

	// test reading from scratch
	s, _ = NewStoreBinaryWithWriter(nil)
	if err = s.readExistingRecords(&ReaderByteReader{&buf}); err != nil {
		t.Fatal(err)
	}
	ts.ensureStateAfter4()
}

// test appending to existing translation log works
func TestTransLog2(t *testing.T) {
	path := "transtest.dat"
	os.Remove(path) // just in case

	ts := NewTranslationLogTestState(t, path)
	ts.ensureStateEmpty()

	ts.writeNewTranslation("foo", "foo-us", "us", "user1")
	ts.ensureStateAfter1()
	ts.s.Close()

	ts = NewTranslationLogTestState(t, path)
	ts.ensureStateAfter1()

	ts.writeNewTranslation("foo", "foo-pl", "pl", "user1")
	ts.ensureStateAfter2()
	ts.s.Close()

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

	ts.s.Close()
}
