package main

import (
	"bytes"
	"fmt"
	"testing"
)

func errIf(t *testing.T, cond bool, msg string) {
	if cond {
		t.Error(msg)
	}
}

func TestTransLog(t *testing.T) {
	var buf bytes.Buffer

	s := NewEncoderDecoderState()
	if len(s.langCodeMap) != 0 {
		t.Fatal("len(s.langCodeMap)=", len(s.langCodeMap), ", expected: 0")
	}
	if len(s.translations) != 0 {
		t.Fatal("len(s.translations)=", len(s.translations), ", expected: 0")
	}

	err := s.writeNewTranslation(&buf, "foo", "foo-us", "us", "user1")
	if err != nil {
		t.Fatal(err)
	}
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

	err = s.writeNewTranslation(&buf, "foo", "foo-pl", "pl", "user1")
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
	if len(s.translations) != 2 {
		t.Fatal("len(s.translations)=", len(s.translations), ", expected: 2")
	}

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
	s = NewEncoderDecoderState()
	r := &ReaderByteReader{&buf}

	err = s.readExistingRecords(r)
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
