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

func ensureStateEmpty(t *testing.T, s *EncoderDecoderState) {
	if len(s.langCodeMap) != 0 {
		t.Fatal("len(s.langCodeMap)=", len(s.langCodeMap), ", expected: 0")
	}
	if len(s.translations) != 0 {
		t.Fatal("len(s.translations)=", len(s.translations), ", expected: 0")
	}
}

func ensureStateAfter1(t *testing.T, s *EncoderDecoderState) {
	if len(s.langCodeMap) != 1 {
		t.Fatal("len(s.langCodeMap)=", len(s.langCodeMap), ", expected: 1")
	}
	if s.langCodeMap["us"] != 1 {
		t.Fatal("s.langCodeMap['us']=", s.langCodeMap["us"], ", expected: 1")
	}
	if len(s.userNameMap) != 1 {
		t.Fatalf("len(s.userNameMap)=%d, expected: 1", len(s.userNameMap))
	}
	if s.userNameMap["user1"] != 1 {
		t.Fatalf("s.userNameMap['user1'], expected: 1", s.userNameMap["user1"])
	}
	if len(s.stringMap) != 1 {
		t.Fatalf("len(s.stringMap)=%d, expected: 1", len(s.stringMap))
	}

	if len(s.translations) != 1 {
		t.Fatal("len(s.translations)=", len(s.translations), ", expected: 1")
	}
}

func ensureStateAfter2(t *testing.T, s *EncoderDecoderState) {
	if len(s.langCodeMap) != 2 {
		t.Fatal("len(s.langCodeMap)=", len(s.langCodeMap), ", expected: 2")
	}
	if s.langCodeMap["pl"] != 2 {
		t.Fatal("s.langCodeMap['pl']=", s.langCodeMap["pl"], ", expected: 2")
	}
	if len(s.userNameMap) != 1 {
		t.Fatalf("len(s.userNameMap)=%d, expected: 1", len(s.userNameMap))
	}
	if len(s.translations) != 2 {
		t.Fatal("len(s.translations)=", len(s.translations), ", expected: 2")
	}
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
	if len(s.translations) != 3 {
		t.Fatal("len(s.translations)=", len(s.translations), ", expected: 3")
	}

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
	if len(s.langCodeMap) != 2 {
		t.Fatal("len(s.langCodeMap)=", len(s.langCodeMap), ", expected: 2")
	}
	if s.langCodeMap["pl"] != 2 {
		t.Fatal("s.langCodeMap['pl']=", s.langCodeMap["pl"], ", expected: 2")
	}
	if len(s.userNameMap) != 1 {
		t.Fatalf("len(s.userNameMap)=%d, expected: 1", len(s.userNameMap))
	}
	if len(s.translations) != 3 {
		t.Fatal("len(s.translations)=", len(s.translations), ", expected: 3")
	}
}
