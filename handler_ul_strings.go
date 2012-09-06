package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type CantParseError struct {
	Msg    string
	LineNo int
}

func (e *CantParseError) Error() string {
	return fmt.Sprintf("Error: %s on line %d", e.Msg, e.LineNo)
}

func myReadLine(r *bufio.Reader) (string, error) {
	line, isPrefix, err := r.ReadLine()
	if err != nil {
		return "", err
	}
	if isPrefix {
		return "", &CantParseError{"Line too long", -1}
	}
	return string(line), nil
}

// returns nil on error
func parseUploadedStrings(reader io.Reader) []string {
	r := bufio.NewReaderSize(reader, 4*1024)
	res := make([]string, 0)
	parsedHeader := false
	for {
		line, err := myReadLine(r)
		if err != nil {
			if err == io.EOF {
				if 0 == len(res) {
					return nil
				}
				return res
			}
			return nil
		}
		if !parsedHeader {
			// first line must be: AppTranslator strings
			if line != "AppTranslator strings" {
				fmt.Printf("parseUploadedStrings(): invalid first line: '%s'\n", line)
				return nil
			}
			parsedHeader = true
		} else {
			if 0 == len(line) {
				fmt.Printf("parseUploadedStrings(): encountered empty line\n")
				return nil
			}
			res = append(res, line)
		}
	}
	return res
}

// handler for url: POST /handleUploadStrings?app=$appName&secret=$uploadSecret
// POST data is in the format:
/*
AppTranslator strings
string to translate 1
string to translate 2
...
*/
func handleUploadStrings(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	secret := strings.TrimSpace(r.FormValue("secret"))
	if secret != app.config.UploadSecret {
		serveErrorMsg(w, fmt.Sprintf("Invalid secret for app '%s'", appName))
		return
	}
	file, _, err := r.FormFile("strings")
	if err != nil {
		serveErrorMsg(w, fmt.Sprintf("No file with strings %s", err.Error()))
		return
	}
	defer file.Close()
	newStrings := parseUploadedStrings(file)
	if nil == newStrings {
		serveErrorMsg(w, "Error parsing uploaded strings")
		return
	}
	w.Write([]byte("Ok\n"))
}
