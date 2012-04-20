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
	LangId          string // iso name of the language ("en", "cn", "sp-rs")
	LangNameEnglish string
	LangNameNative  string
	Strings         []string
	Translations    []string
}

func NewLangTranslations() (lt *LangTranslations) {
	lt = new(LangTranslations)
	lt.Strings = make([]string, 0)
	lt.Translations = make([]string, 0)
	return
}

func (lt *LangTranslations) Add(str, trans string) {
	lt.Strings = append(lt.Strings, str)
	lt.Translations = append(lt.Translations, trans)
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

func Parse(reader io.Reader) (*LangTranslations, error) {
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
				return lt, nil
			}
			//err.LineNo = lineNo
			return nil, err
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
					lt.LangId, lt.LangNameEnglish, lt.LangNameNative = parseLang(val)
					if "" == lt.LangId {
						msg := fmt.Sprintf("Couldn't parse '%s'", s)
						return nil, &CantParseError{msg, lineNo}
					}
				} else {
					msg := fmt.Sprintf("Enexpected header: '%s'", name)
					return nil, &CantParseError{msg, lineNo}
				}
			}
			continue
		}

		if ParsingAfterString == state {
			if isEmptyOrComment(s) {
				msg := "Unexpected empty or comment line"
				return nil, &CantParseError{msg, lineNo}
			}
			lt.Add(currString, s)
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
	return lt, nil
}

func ParseFile(name string) (*LangTranslations, error) {
	file, err := os.Open(name)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	return Parse(file)
}
