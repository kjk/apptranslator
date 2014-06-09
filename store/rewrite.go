package store

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

var (
	stdoutCsvWriter *csv.Writer
	csvWriter       *csv.Writer
	internedStrings *StringInterner
)

func writeCsv(record []string) {
	recs := [][]string{record}
	if err := csvWriter.WriteAll(recs); err != nil {
		log.Fatalf("writeCsv: csv.Writer.WriteAll() failed with '%s'", err)
	}
	if stdoutCsvWriter != nil {
		if err := stdoutCsvWriter.WriteAll(recs); err != nil {
			log.Fatalf("writeCsv: csv.Writer.WriteAll() failed with '%s'", err)
		}
	}
}

func RewriteStore(binaryPath, csvPath string) {
	fmt.Printf("RewriteStore('%s', '%s')\n", binaryPath, csvPath)
	logging = true
	dst, err := os.Create(csvPath)
	if err != nil {
		log.Fatalf("os.Create('%s') failed with '%s'", csvPath, err)
	}
	defer dst.Close()

	internedStrings = NewStringInterner()
	csvWriter = csv.NewWriter(dst)
	csvWriter.Comma = ','
	defer csvWriter.Flush()

	stdoutCsvWriter = csv.NewWriter(os.Stdout)
	stdoutCsvWriter.Comma = ','
	defer stdoutCsvWriter.Flush()

	s, err := NewStoreBinary(binaryPath)
	if err != nil {
		log.Fatalf("NewStoreBinary() failed with '%s'", err)
	}

	activeStrings := s.getActiveStrings()
	timeSecsStr := strconv.FormatInt(time.Now().Unix(), 10)
	n := len(activeStrings) + 2
	rec := make([]string, n, n)
	rec[0] = recIdActiveSet
	rec[1] = timeSecsStr
	fmt.Printf("Active strings (%d):\n", len(activeStrings))
	for i := 0; i < len(activeStrings); i++ {
		str := activeStrings[i]
		strId, isNew := internedStrings.Intern(str)
		panicIf(isNew, "isNew is true")
		rec[2+i] = strconv.Itoa(strId)
		fmt.Printf("  '%s'\n", str)
	}
	writeCsv(rec)
	deleted := s.GetDeletedStrings()
	fmt.Printf("Deleted strings (%d):\n", len(deleted))
	for _, str := range deleted {
		fmt.Printf("  '%s'\n", str)
	}
	s.Close()
}
