// This code is under BSD license. See license-bsd.txt
package store

import (
	"os"
	"testing"
)

func (s *StoreCsv) writeNewTranslationMust(s1, s2, s3, s4 string) {
	err := s.WriteNewTranslation(s1, s2, s3, s4)
	fatalIf(err != nil, "err != nil")
}

func (s *StoreCsv) duplicateTranslationMust(origStr, newStr string) {
	err := s.DuplicateTranslation(origStr, newStr)
	fatalIf(err != nil, "err != nil")
}

func (s *StoreCsv) updateStringsListMust(sl []string) ([]string, []string, []string) {
	added, deleted, undeleted, err := s.UpdateStringsList(sl)
	fatalIf(err != nil, "err != nil")
	return added, deleted, undeleted
}

func (s *StoreCsv) ensureUserCount(exp int) {
	fatalIf(s.users.Count() != exp, "len(s.userNameMap)=%d, exp: %d", s.users.Count(), exp)
}

func (s *StoreCsv) ensureStringsCount(exp int) {
	fatalIf(s.strings.Count() != exp, "len(s.stringMap)=%d, exp: %d", s.strings.Count(), exp)
}

func (s *StoreCsv) ensureUndeletedStringsCount(exp int) {
	n := s.activeStringsCount()
	fatalIf(n != exp, "len(s.activeStringsCount())=%d, exp: %d", s.activeStringsCount(), exp)
}

func (s *StoreCsv) ensureTranslationsCount(exp int) {
	fatalIf(len(s.edits) != exp, "len(s.translations)=%d, exp: %d", len(s.edits), exp)
}

func (s *StoreCsv) ensureLangCode(name string, exp int) {
	// TODO: write me
	//fatalIf(s.langCodeMap[name] != exp, "s.langCodeMap[%q]=%d, exp: %d", name, s.langCodeMap[name], exp)
}

func (s *StoreCsv) ensureUserCode(user string, exp int) {
	userId := s.users.IdByStrMust(user)
	fatalIf(userId != exp, "user: %q, got: %d, exp: %d", user, userId, exp)
}

func (s *StoreCsv) ensureStringCode(str string, exp int) {
	strId := s.strings.IdByStrMust(str)
	fatalIf(strId != exp, "str: %q, got: %d, exp: %d", str, strId, exp)
}

func (s *StoreCsv) ensureDeletedCount(exp int) {
	// TODO: write me
	//fatalIf(len(s.deletedStrings) != exp, "len(s.deletedStrings)=%d, exp: %d", len(s.deletedStrings), exp)
}

func (s *StoreCsv) ensureStateEmpty() {
	s.ensureUserCount(0)
	s.ensureStringsCount(0)
}

func (s *StoreCsv) ensureStateAfter1() {
	s.ensureLangCode("uk", 1)
	s.ensureUserCount(1)
	s.ensureUserCode("user1", 0)
	s.ensureStringsCount(1)
	s.ensureTranslationsCount(1)
}

func (s *StoreCsv) ensureStateAfter2() {
	s.ensureLangCode("pl", 2)
	s.ensureUserCount(1)
	s.ensureTranslationsCount(2)
}

func (s *StoreCsv) ensureStateAfter3() {
	s.ensureUserCount(1)
	s.ensureLangCode("uk", 1)
	s.ensureUserCode("user1", 0)
	s.ensureLangCode("pl", 2)
	s.ensureTranslationsCount(3)
}

func (s *StoreCsv) ensureStateAfter4() {
	s.ensureUserCount(1)
	s.ensureLangCode("uk", 1)
	s.ensureUserCode("user1", 0)
	s.ensureLangCode("pl", 2)
	s.ensureTranslationsCount(5)
}

func (s *StoreCsv) ensureStringsAre(strs []string) {
	s.ensureUndeletedStringsCount(len(strs))
	for _, str := range strs {
		s.strings.IdByStrMust(str)
	}
}

func NewTestStore(path string) *StoreCsv {
	s, err := NewStoreCsv(path)
	fatalIf(err != nil, "Failed to create new transtest.dat")
	return s
}

func TestTransLog(t *testing.T) {
	path := "transtest.dat"
	os.Remove(path) // just in case
	s := NewTestStore(path)
	s.ensureStateEmpty()

	s.writeNewTranslationMust("foo", "foo-uk", "uk", "user1")
	s.ensureStateAfter1()

	s.writeNewTranslationMust("foo", "foo-pl", "pl", "user1")
	s.ensureStateAfter2()

	s.writeNewTranslationMust("bar", "bar-pl", "pl", "user1")
	s.ensureStateAfter3()

	s.duplicateTranslationMust("foo", "foo2")
	s.ensureStateAfter4()
}

// test appending to existing translation log works
func TestTransLog2(t *testing.T) {
	path := "transtest.dat"
	os.Remove(path) // just in case

	s := NewTestStore(path)
	s.ensureStateEmpty()

	s.writeNewTranslationMust("foo", "foo-uk", "uk", "user1")
	s.ensureStateAfter1()
	s.Close()

	s = NewTestStore(path)
	s.ensureStateAfter1()

	s.writeNewTranslationMust("foo", "foo-pl", "pl", "user1")
	s.ensureStateAfter2()
	s.Close()

	s = NewTestStore(path)
	s.ensureStateAfter2()

	s.updateStringsListMust([]string{"foo", "bar", "go"})
	//added, deleted, undeleted := s.updateStringsListMust([]string{"foo", "bar", "go"})
	//fatalIf(len(added) != 2, "len(added) = %d != 2", len(added))
	//fatalIf(len(deleted) != 0, "len(deleted) = %d != 0", len(deleted))
	//fatalIf(len(undeleted) != 0, "len(undeleted) = %d != 0", len(undeleted))
	s.ensureStringsAre([]string{"foo", "bar", "go"})

	_, _, _ = s.updateStringsListMust([]string{"foo", "bar"})
	//fatalIf(len(deleted) != 1, "len(deleted) != 1")

	_, _, _ = s.updateStringsListMust([]string{"foo", "bar", "go"})
	//fatalIf(len(undeleted) != 1, "len(undeleted) != 1")

	s.Close()
}
