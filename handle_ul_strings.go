// This code is under BSD license. See license-bsd.txt
package main

import (
	"errors"
	"fmt"
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

func normalizeNewlines(s string) string {
	s = strings.Replace(s, "\n\r", "\n", -1)
	return strings.Replace(s, "\r", "\n", -1)
}

func parseUploadedStrings(s string) ([]string, error) {
	s = normalizeNewlines(s)
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return nil, errors.New("not enough lines")
	}
	if lines[0] != "AppTranslator strings" {
		return nil, errors.New("First line is not 'Apptranslator strings'")
	}
	lines = lines[1:]
	for _, l := range lines {
		if 0 == len(l) {
			return nil, errors.New("found empty line")
		}
	}
	return lines, nil
}

// url: POST /uploadstrings?app=$appName&secret=$uploadSecret
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
		logger.Noticef("Someone tried to upload strings for non-existing app %s", appName)
		serveErrorMsg(w, fmt.Sprintf("Application %q doesn't exist", appName))
		return
	}
	secret := strings.TrimSpace(r.FormValue("secret"))
	if secret != app.UploadSecret {
		logger.Noticef("Someone tried to upload strings for %s with invalid secret %s", appName, secret)
		serveErrorMsg(w, fmt.Sprintf("Invalid secret for app %q", appName))
		return
	}
	s := r.FormValue("strings")
	if newStrings, err := parseUploadedStrings(s); err != nil {
		logger.Noticef("parseUploadedStrings() failed with %s", err)
		serveErrorMsg(w, "Error parsing uploaded strings")
		return
	} else {
		logger.Noticef("handleUploadString(): uploading %d strings for %s", len(newStrings), appName)
		added, deleted, undeleted, err := app.store.UpdateStringsList(newStrings)
		if err != nil {
			logger.Errorf("UpdateStringsList() failed with %s", err)
		} else {
			msg := ""
			if len(added) > 0 {
				msg += fmt.Sprintf("New strings: %v\n", added)
			}
			if len(deleted) > 0 {
				msg += fmt.Sprintf("Deleted strings: %v\n", deleted)
			}
			if len(undeleted) > 0 {
				msg += fmt.Sprintf("Undeleted strings: %v\n", undeleted)
			}
			if len(msg) > 0 {
				logger.Notice(msg)
			}
			w.Write([]byte(msg))
		}
	}
}
