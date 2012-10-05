// This code is under BSD license. See license-bsd.txt
package main

import (
	"bufio"
	_ "encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"bytes"
)

const (
	_ = iota

	ParsingMeta
	ParsingBeforeString
	ParsingAfterString
)

type langTranslations struct {
	LangCode        string // iso name of the language ("en", "cn", "sp-rs")	
	LangNameEnglish string
	LangNameNative  string
}

type CantParseError struct {
	Msg    string
	LineNo int
}

func (e *CantParseError) Error() string {
	return fmt.Sprintf("Error: %s on line %d", e.Msg, e.LineNo)
}

func isComment(s string) bool {
	if 0 == strings.Index(s, "#") {
		return true
	}
	return false
}

func isEmptyOrComment(s string) bool {
	if 0 == len(s) {
		return true
	}
	return isComment(s)
}

func parseAsNameValue(s string) (string, string) {
	parts := strings.SplitN(s, ":", 2)
	if 1 == len(parts) {
		return "", ""
	}
	name := parts[0]
	val := strings.TrimLeft(parts[1], " ")
	return name, val
}

func myReadLine(r *bufio.Reader) ([]byte, error) {
	line, isPrefix, err := r.ReadLine()
	if err != nil {
		return nil, err
	}
	if isPrefix {
		return nil, &CantParseError{"Line too long", -1}
	}
	return line, nil
}

func removeBom(b []byte) []byte {
	if len(b) >= 3 {
		if b[0] == 0xef && b[1] == 0xbb && b[2] == 0xbf {
			return b[3:]
		}
	}
	return b
}

// given s as:
// cn Chinese Simplified (简体中文)
// returns "cn" as id, "Chinese Simplified" as nameEnglish and "简体中文" as nameNative
func parseLang(s string) (id, nameEnglish, nameNative string) {
	parts := strings.SplitN(s, " ", 2)
	if len(parts) != 2 {
		return "", "", ""
	}
	id = parts[0]
	parts = strings.SplitN(parts[1], "(", 2)
	if len(parts) != 2 {
		name := strings.Trim(parts[0], " ")
		return id, name, name
	}
	nameEnglish = strings.Trim(parts[0], " ")
	nameNative = strings.TrimRight(parts[1], " )")
	return
}

func parseSumatraTranslationsFile(fileName string, tl *TranslationLog) error {
	encoder := tl.state
	reader, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer reader.Close()

	r := bufio.NewReaderSize(reader, 4*1024)
	lt := new(langTranslations)
	state := ParsingMeta
	currString := ""
	lineNo := 0
	for {
		lineNo++
		line, err := myReadLine(r)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			//err.LineNo = lineNo
			return err
		}
		line = bytes.TrimRight(removeBom(line), " \r\n")
		s := string(line)
		if ParsingMeta == state {
			if isEmptyOrComment(s) {
				continue
			}
			name, val := parseAsNameValue(s)
			if "" == name {
				currString = s
				state = ParsingAfterString
			} else {
				if "Contributor" == name {
					// do nothing
				} else if "Lang" == name {
					lt.LangCode, lt.LangNameEnglish, lt.LangNameNative = parseLang(val)
					if "" == lt.LangCode {
						msg := fmt.Sprintf("Couldn't parse '%s'", s)
						return &CantParseError{msg, lineNo}
					}
				} else {
					msg := fmt.Sprintf("Enexpected header: '%s'", name)
					return &CantParseError{msg, lineNo}
				}
			}
			continue
		}

		if ParsingAfterString == state {
			if isEmptyOrComment(s) {
				msg := "Unexpected empty or comment line"
				return &CantParseError{msg, lineNo}
			}
			err := encoder.writeNewTranslation(tl.file, currString, s, lt.LangCode, "import")
			if nil != err {
				fmt.Printf("Error in file %s line %d\n", fileName, lineNo)
			}
			state = ParsingBeforeString
			continue
		}

		if ParsingBeforeString == state {
			if isEmptyOrComment(s) {
				continue
			}
			currString = s
			state = ParsingAfterString
			continue
		}

		panic("Unexpected parsing state")
	}
	return nil
}

var dataFileName = "SumatraPDF_trans.dat"

func main() {
	dir := "../sumatrapdf/strings"
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Printf("Error reading dir '%s', %s\n", dir, err.Error())
		return
	}
	if PathExists(dataFileName) {
		log.Fatalf("%s already exists", dataFileName)
	}
	translog, err := NewTranslationLog(dataFileName)
	if translog == nil {
		return
	}
	defer translog.close()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		name := e.Name()
		langCode := name[:len(name)-4]
		if !IsValidLangCode(langCode) {
			log.Fatalf("'%s' is not a valid language code\n", langCode)
		}
		path := filepath.Join(dir, e.Name())
		parseSumatraTranslationsFile(path, translog)
	}
}
