// This code is under BSD license. See license-bsd.txt
package store

import (
	"bytes"
	"os"
	"testing"
)

type TestState struct {
	t *testing.T
	// ether l is set or both s and w
	s *StoreBinary
}

func (ts *TestState) writeNewTranslation(s1, s2, s3, s4 string) {
	err := ts.s.WriteNewTranslation(s1, s2, s3, s4)
	panicif(err != nil)
}

func (ts *TestState) DuplicateTranslation(origStr, newStr string) {
	err := ts.s.DuplicateTranslation(origStr, newStr)
	panicif(err != nil)
}

func (ts *TestState) deleteString(str string) {
	err := ts.s.deleteString(ts.s.w, str)
	panicif(err != nil)
}

func (ts *TestState) undeleteString(str string) {
	err := ts.s.undeleteString(ts.s.w, str)
	panicif(err != nil)
}

func (ts *TestState) UpdateStringsList(s []string) ([]string, []string, []string) {
	added, deleted, undeleted, err := ts.s.UpdateStringsList(s)
	panicif(err != nil)
	return added, deleted, undeleted
}

func (ts *TestState) ensureLangCount(exp int) {
	s := ts.s
	panicif(len(s.langCodeMap) != exp, "len(s.langCodeMap)=%d, exp: %d", len(s.langCodeMap), exp)
}

func (ts *TestState) ensureUserCount(exp int) {
	s := ts.s
	panicif(len(s.userNameMap) != exp, "len(s.userNameMap)=%d, exp: %d", len(s.userNameMap), exp)
}

func (ts *TestState) ensureStringsCount(exp int) {
	s := ts.s
	panicif(len(s.stringMap) != exp, "len(s.stringMap)=%d, exp: %d", len(s.stringMap), exp)
}

func (ts *TestState) ensureUndeletedStringsCount(exp int) {
	s := ts.s
	n := s.activeStringsCount()
	panicif(n != exp, "len(s.activeStringsCount())=%d, exp: %d", s.activeStringsCount(), exp)
}

func (ts *TestState) ensureTranslationsCount(exp int) {
	s := ts.s
	panicif(len(s.edits) != exp, "len(s.translations)=%d, exp: %d", len(s.edits), exp)
}

func (ts *TestState) ensureLangCode(name string, exp int) {
	s := ts.s
	panicif(s.langCodeMap[name] != exp, "s.langCodeMap[%q]=%d, exp: %d", name, s.langCodeMap[name], exp)
}

func (ts *TestState) ensureUserCode(name string, exp int) {
	s := ts.s
	panicif(s.userNameMap[name] != exp, "s.userNameMap[%q]=%d, exp: %d", name, s.userNameMap[name], exp)
}

func (ts *TestState) ensureStringCode(name string, exp int) {
	s := ts.s
	panicif(s.stringMap[name] != exp, "s.stringMap[%q]=%d, exp: %d", name, s.stringMap[name], exp)
}

func (ts *TestState) ensureDeletedCount(exp int) {
	s := ts.s
	panicif(len(s.deletedStrings) != exp, "len(s.deletedStrings)=%d, exp: %d", len(s.deletedStrings), exp)
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
		_, exist := s.stringMap[str]
		panicif(!exist, "%q doesn't exist in s.stringMap", str)
	}
}

func NewStoreTestState(t *testing.T, path string) *TestState {
	s, err := NewStoreBinary(path)
	panicif(err != nil, "Failed to create new transtest.dat")
	return &TestState{t, s}
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

	ts := NewStoreTestState(t, path)
	ts.ensureStateEmpty()

	ts.writeNewTranslation("foo", "foo-us", "us", "user1")
	ts.ensureStateAfter1()
	ts.s.Close()

	ts = NewStoreTestState(t, path)
	ts.ensureStateAfter1()

	ts.writeNewTranslation("foo", "foo-pl", "pl", "user1")
	ts.ensureStateAfter2()
	ts.s.Close()

	ts = NewStoreTestState(t, path)
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
