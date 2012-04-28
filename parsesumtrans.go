package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	_ = iota

	ParsingMeta
	ParsingBeforeString
	ParsingAfterString
)

type LangTranslations struct {
	LangCode        string	// iso name of the language ("en", "cn", "sp-rs")	
	LangNameEnglish string
	LangNameNative  string
}

func NewLangTranslations() (lt *LangTranslations) {
	lt = new(LangTranslations)
	return
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

func isTrailing(b byte) bool {
	switch b {
	case ' ', '\n', '\r':
		return true
	}
	return false
}

func removeTrailing(b []byte) []byte {
	i := len(b) - 1
	for i >= 0 {
		if !isTrailing(b[i]) {
			break
		}
		i--
	}
	if i < len(b)-1 {
		return b[0:i]
	}
	return b
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

func ParseFile(fileName string, tl *TranslationLog) error {
	reader, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer reader.Close()

	r := bufio.NewReaderSize(reader, 4*1024)
	lt := NewLangTranslations()
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
		line = removeTrailing(removeBom(line))
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
			err := tl.write(lt.LangCode, currString, s)
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
