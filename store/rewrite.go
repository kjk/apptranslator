package store

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
)

var (
	stdoutCsvWriter *csv.Writer
	csvWriter       *csv.Writer
	internedStrings *StringInterner
)

func writeCsv(record []string) {
	recs := [][]string{record}
	if err := csvWriter.WriteAll(recs); err != nil {
		log.Fatalf("writeCsv: csv.Writer.WriteAll() failed with %q", err)
	}
	if stdoutCsvWriter != nil {
		if err := stdoutCsvWriter.WriteAll(recs); err != nil {
			log.Fatalf("writeCsv: csv.Writer.WriteAll() failed with %q", err)
		}
	}
}

func RewriteStore(binaryPath, csvPath string) {
	fmt.Printf("RewriteStore(%q, %q)\n", binaryPath, csvPath)
	//logging = true
	dst, err := os.Create(csvPath)
	if err != nil {
		log.Fatalf("os.Create(%q) failed with %q", csvPath, err)
	}
	defer dst.Close()

	internedStrings = NewStringInterner()
	csvWriter = csv.NewWriter(dst)
	csvWriter.Comma = ','
	defer csvWriter.Flush()

	/*stdoutCsvWriter = csv.NewWriter(os.Stdout)
	stdoutCsvWriter.Comma = ','
	defer stdoutCsvWriter.Flush()*/

	s, err := NewStoreBinary(binaryPath)
	if err != nil {
		log.Fatalf("NewStoreBinary() failed with %q", err)
	}

	activeStrings := s.getActiveStrings()
	n := len(activeStrings)
	activeStringsInt := make([]int, n, n)
	for i := 0; i < n; i++ {
		str := activeStrings[i]
		strId, isNew := internedStrings.Intern(str)
		panicIf(isNew, "isNew is true")
		activeStringsInt[i] = strId
	}
	rec := buildActiveSetRec(activeStringsInt)
	writeCsv(rec)
	/*deleted := s.GetDeletedStrings()
	fmt.Printf("Deleted strings (%d):\n", len(deleted))
	for _, str := range deleted {
		fmt.Printf("  %q\n", str)
	}*/
	s.Close()
}
