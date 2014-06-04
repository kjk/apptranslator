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
)

const (
	recIdTrans = "t"
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
	s.Close()
}
