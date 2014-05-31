package store

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"time"
)

var (
	langCodeMap map[string]int
	userNameMap map[string]int
	stringMap   map[string]int
)

func decodeStringDeleteRecord(rec []byte) {
	id, n := binary.Uvarint(rec)
	panicIf(n != len(rec), "decodeStringDeleteRecord")
	txt := stringById(stringMap, int(id))
	fmt.Printf("Delete: %s\n", txt)
}

func decodeStringUndeleteRecord(rec []byte) {
	id, n := binary.Uvarint(rec)
	panicIf(n != len(rec), "decodeStringUndeleteRecord")
	txt := stringById(stringMap, int(id))
	fmt.Printf("Undelete: %s\n", txt)
}

func decodeNewTranslation(rec []byte, time time.Time) {
	var stringId, userId, langId uint64
	var n int

	langId, n = binary.Uvarint(rec)
	panicIf(n <= 0 || n == len(rec), "decodeNewTranslation() langId")
	//panicIf(!s.validLangId(int(langId)), "decodeNewTranslation(): !s.validLangId()")
	rec = rec[n:]

	userId, n = binary.Uvarint(rec)
	panicIf(n == len(rec), "decodeNewTranslation() userId")
	//panicIf(!s.validUserId(int(userId)), "decodeNewTranslation(): !s.validUserId()")
	rec = rec[n:]

	stringId, n = binary.Uvarint(rec)
	panicIf(n == 0 || n == len(rec), "decodeNewTranslation() stringId")
	//panicIf(!s.validStringId(int(stringId)), fmt.Sprintf("decodeNewTranslation(): !s.validStringId(%v)", stringId))
	rec = rec[n:]

	translation := string(rec)
	fmt.Printf("newTrans: %d, %d, %d, %s, %d\n", int(langId), int(userId), int(stringId), translation, time.Unix())
}

func decodeRecord(rec []byte, time time.Time) {
	panicIf(len(rec) < 2, "decodeRecord(), len(rec) < 2")
	if 0 == rec[0] {
		t := rec[1]
		switch t {
		case newLangIdRec:
			decodeStrRecord(rec[2:], langCodeMap, "langCodeMap")
		case newUserNameIdRec:
			decodeStrRecord(rec[2:], userNameMap, "userNameMap")
		case newStringIdRec:
			decodeStrRecord(rec[2:], stringMap, "stringMap")
		case strDelRec:
			decodeStringDeleteRecord(rec[2:])
		case strUndelRec:
			decodeStringUndeleteRecord(rec[2:])
		default:
			log.Fatalf("Unexpected t=%d", t)
		}
	} else {
		decodeNewTranslation(rec, time)
	}
}

func readRecords(r *ReaderByteReader) {
	nRecs := 0
	for {
		buf, time, err := readRecord(r)
		if err != nil {
			log.Fatalf("readRecords: readRecord failed with '%s'\n", err)
		}
		if buf == nil {
			return
		}
		decodeRecord(buf, time)
		nRecs += 1
	}
	fmt.Printf("readRecord: %d records\n", nRecs)
}

func RewriteStore(binaryPath, csvPath string) {
	fmt.Printf("RewriteStore('%s', '%s')\n", binaryPath, csvPath)
	langCodeMap = make(map[string]int)
	userNameMap = make(map[string]int)
	stringMap = make(map[string]int)

	src, err := os.Open(binaryPath)
	if err != nil {
		log.Fatalf("RewriteStore: os.Open('%s') failed with '%s'", binaryPath, err)
	}
	defer src.Close()
	r := &ReaderByteReader{src}
	readRecords(r)
}
