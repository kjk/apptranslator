// This code is under BSD license. See license-bsd.txt
package store

import (
	"bytes"
	"os"
	"testing"
)

func (s *StoreBinary) writeNewTranslationMust(s1, s2, s3, s4 string) {
	err := s.WriteNewTranslation(s1, s2, s3, s4)
	panicif(err != nil)
}

func (s *StoreBinary) duplicateTranslationMust(origStr, newStr string) {
	err := s.DuplicateTranslation(origStr, newStr)
	panicif(err != nil)
}

func (s *StoreBinary) deleteStringMust(str string) {
	err := s.deleteString(s.w, str)
	panicif(err != nil)
}

func (s *StoreBinary) undeleteStringMust(str string) {
	err := s.undeleteString(s.w, str)
	panicif(err != nil)
}

func (s *StoreBinary) updateStringsListMust(sl []string) ([]string, []string, []string) {
	added, deleted, undeleted, err := s.UpdateStringsList(sl)
	panicif(err != nil)
	return added, deleted, undeleted
}

func (s *StoreBinary) ensureLangCount(exp int) {
	panicif(len(s.langCodeMap) != exp, "len(s.langCodeMap)=%d, exp: %d", len(s.langCodeMap), exp)
}

func (s *StoreBinary) ensureUserCount(exp int) {
	panicif(len(s.userNameMap) != exp, "len(s.userNameMap)=%d, exp: %d", len(s.userNameMap), exp)
}

func (s *StoreBinary) ensureStringsCount(exp int) {
	panicif(len(s.stringMap) != exp, "len(s.stringMap)=%d, exp: %d", len(s.stringMap), exp)
}

func (s *StoreBinary) ensureUndeletedStringsCount(exp int) {
	n := s.activeStringsCount()
	panicif(n != exp, "len(s.activeStringsCount())=%d, exp: %d", s.activeStringsCount(), exp)
}

func (s *StoreBinary) ensureTranslationsCount(exp int) {
	panicif(len(s.edits) != exp, "len(s.translations)=%d, exp: %d", len(s.edits), exp)
}

func (s *StoreBinary) ensureLangCode(name string, exp int) {
	panicif(s.langCodeMap[name] != exp, "s.langCodeMap[%q]=%d, exp: %d", name, s.langCodeMap[name], exp)
}

func (s *StoreBinary) ensureUserCode(name string, exp int) {
	panicif(s.userNameMap[name] != exp, "s.userNameMap[%q]=%d, exp: %d", name, s.userNameMap[name], exp)
}

func (s *StoreBinary) ensureStringCode(name string, exp int) {
	panicif(s.stringMap[name] != exp, "s.stringMap[%q]=%d, exp: %d", name, s.stringMap[name], exp)
}

func (s *StoreBinary) ensureDeletedCount(exp int) {
	panicif(len(s.deletedStrings) != exp, "len(s.deletedStrings)=%d, exp: %d", len(s.deletedStrings), exp)
}

func (s *StoreBinary) ensureStateEmpty() {
	s.ensureLangCount(0)
	s.ensureUserCount(0)
	s.ensureStringsCount(0)
}

func (s *StoreBinary) ensureStateAfter1() {
	s.ensureLangCount(1)
	s.ensureLangCode("us", 1)
	s.ensureUserCount(1)
	s.ensureUserCode("user1", 1)
	s.ensureStringsCount(1)
	s.ensureTranslationsCount(1)
}

func (s *StoreBinary) ensureStateAfter2() {
	s.ensureLangCount(2)
	s.ensureLangCode("pl", 2)
	s.ensureUserCount(1)
	s.ensureTranslationsCount(2)
}

func (s *StoreBinary) ensureStateAfter3() {
	s.ensureLangCount(2)
	s.ensureUserCount(1)
	s.ensureLangCode("us", 1)
	s.ensureUserCode("user1", 1)
	s.ensureLangCode("pl", 2)
	s.ensureTranslationsCount(3)
}

func (s *StoreBinary) ensureStateAfter4() {
	s.ensureLangCount(2)
	s.ensureUserCount(1)
	s.ensureLangCode("us", 1)
	s.ensureUserCode("user1", 1)
	s.ensureLangCode("pl", 2)
	s.ensureTranslationsCount(5)
}

func (s *StoreBinary) ensureStringsAre(strs []string) {
	s.ensureUndeletedStringsCount(len(strs))
	for _, str := range strs {
		_, exist := s.stringMap[str]
		panicif(!exist, "%q doesn't exist in s.stringMap", str)
	}
}

func NewTestStore(path string) *StoreBinary {
	s, err := NewStoreBinary(path)
	panicif(err != nil, "Failed to create new transtest.dat")
	return s
}

func TestTransLog(t *testing.T) {
	var buf bytes.Buffer

	s, err := NewStoreBinaryWithWriter(&buf)
	s.ensureStateEmpty()

	s.writeNewTranslationMust("foo", "foo-us", "us", "user1")
	s.ensureStateAfter1()

	s.writeNewTranslationMust("foo", "foo-pl", "pl", "user1")
	s.ensureStateAfter2()

	s.writeNewTranslationMust("bar", "bar-pl", "pl", "user1")
	s.ensureStateAfter3()

	s.duplicateTranslationMust("foo", "foo2")
	s.ensureStateAfter4()

	s.deleteStringMust("bar")
	s.ensureDeletedCount(1)
	s.ensureUndeletedStringsCount(2)

	s.undeleteStringMust("bar")
	s.ensureDeletedCount(0)
	s.ensureUndeletedStringsCount(3)

	// test reading from scratch
	s, _ = NewStoreBinaryWithWriter(nil)
	err = s.readExistingRecords(&ReaderByteReader{&buf})
	panicif(err != nil)
	s.ensureStateAfter4()
}

// test appending to existing translation log works
func TestTransLog2(t *testing.T) {
	path := "transtest.dat"
	os.Remove(path) // just in case

	s := NewTestStore(path)
	s.ensureStateEmpty()

	s.writeNewTranslationMust("foo", "foo-us", "us", "user1")
	s.ensureStateAfter1()
	s.Close()

	s = NewTestStore(path)
	s.ensureStateAfter1()

	s.writeNewTranslationMust("foo", "foo-pl", "pl", "user1")
	s.ensureStateAfter2()
	s.Close()

	s = NewTestStore(path)
	s.ensureStateAfter2()

	added, deleted, undeleted := s.updateStringsListMust([]string{"foo", "bar", "go"})
	panicif(len(added) != 2, "len(added) != 2")
	panicif(len(deleted) != 0, "len(deleted) != 0")
	panicif(len(undeleted) != 0, "len(undeleted) != 0")
	s.ensureStringsAre([]string{"foo", "bar", "go"})

	_, deleted, _ = s.updateStringsListMust([]string{"foo", "bar"})
	panicif(len(deleted) != 1, "len(deleted) != 1")

	_, _, undeleted = s.updateStringsListMust([]string{"foo", "bar", "go"})
	panicif(len(undeleted) != 1, "len(undeleted) != 1")

	s.Close()
}
