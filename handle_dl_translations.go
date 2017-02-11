// This code is under BSD license. See license-bsd.txt
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

type LangTrans struct {
	lang  string
	trans string
}

// escape newlines with C-style escape codes. if we don't do that,
func escapeTrans(s string) string {
	s = strings.Replace(s, "\n", "\\n", -1)
	s = strings.Replace(s, "\r", "\\r", -1)
	return s
}

func translationsForApp(app *App) []byte {
	m := make(map[string][]LangTrans)
	langInfos := app.store.LangInfos()
	for _, li := range langInfos {
		code := li.Code
		for _, t := range li.ActiveStrings {
			if "" == t.Current() {
				continue
			}
			s := t.String
			l, exists := m[s]
			if !exists {
				l = make([]LangTrans, 0)
			}
			var lt LangTrans
			lt.lang = code
			lt.trans = t.Current()
			l = append(l, lt)
			m[s] = l
		}
	}

	var w bytes.Buffer
	// get strings in sorted order so that we can generate stable output
	strings := make([]string, len(m), len(m))
	i := 0
	for k := range m {
		strings[i] = k
		i++
	}
	sort.Strings(strings)

	for _, s := range strings {
		ltarr := m[s]
		io.WriteString(&w, fmt.Sprintf(":%s\n", s))
		n := len(ltarr)
		// TODO: to be more efficient, allocate translations array outside of loop
		translations := make([]string, n, n)
		for _, lt := range ltarr {
			n -= 1
			translations[n] = fmt.Sprintf("%s:%s\n", lt.lang, escapeTrans(lt.trans))
		}
		sort.Strings(translations)
		for _, trans := range translations {
			io.WriteString(&w, trans)
		}
	}
	return w.Bytes()
}

// url: /dltrans?app=$app&sha1=$sha1
// Returns plain/text response in the format designed for easy parsing:
/*
AppTranslator: $appName
$sha1
:string 1
cv:translation for cv language
pl:translation for pl language
:string 2
pl:translation for pl language
*/
func handleDownloadTranslations(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	sha1In := strings.TrimSpace(r.FormValue("sha1"))
	app := findApp(appName)
	if app == nil {
		httpErrorf(w, "Application %q doesn't exist", appName)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, fmt.Sprintf("AppTranslator: %s\n", app.Name))
	if len(sha1In) != 40 {
		io.WriteString(w, "Error: no sha1 provided\n")
		return
	}
	b := translationsForApp(app)
	sha1 := sha1HexOfBytes(b)
	sha2 := sha1HexOfBytes(b)
	if sha1 != sha2 {
		logger.Errorf("sha1 != sha2 (%s != %s)", sha1, sha2)
	}
	if sha1 == sha1In {
		io.WriteString(w, "No change\n")
		logger.Noticef("Translations download for %s with sha1 %s, didn't change", appName, sha1In)
		return
	}
	io.WriteString(w, fmt.Sprintf("%s\n", sha1))
	w.Write(b)
	logger.Noticef("Translations download for %s with sha1 %s, our sha1 %s", appName, sha1In, sha1)
}
