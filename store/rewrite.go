package store

import (
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
			log.Fatalf("strDelRec not implemented yet")
			//return s.decodeStringDeleteRecord(rec[2:])
		case strUndelRec:
			log.Fatalf("strUndelRec not implemented yet")
			//return s.decodeStringUndeleteRecord(rec[2:])
		default:
			log.Fatalf("Unexpected t=%d", t)
		}
	} else {
		log.Fatalf("decodeNewTranslation not implemented yet")
		//return s.decodeNewTranslation(rec, time)
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
