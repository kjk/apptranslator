// This code is under BSD license. See license-bsd.txt
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type LangTrans struct {
	lang  string
	trans string
}

func translationsForApp(app *App) []byte {
	m := make(map[string][]LangTrans)
	langInfos := app.translationLog.LangInfos()
	for _, li := range langInfos {
		code := li.Code
		for _, t := range li.Translations {
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
	for s, ltarr := range m {
		io.WriteString(&w, fmt.Sprintf(":%s\n", s))
		for _, lt := range ltarr {
			io.WriteString(&w, fmt.Sprintf("%s:%s\n", lt.lang, lt.trans))
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
		serveErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, fmt.Sprintf("AppTranslator: %s\n", app.Name))
	if len(sha1In) != 40 {
		io.WriteString(w, "Error: no sha1 provided\n")
		return
	}
	b := translationsForApp(app)
	sha1, err := DataSha1(b)
	if err != nil {
		serveErrorMsg(w, fmt.Sprintf("Error from DataSha1 %s", err.Error()))
		return
	}
	if sha1 == sha1In {
		io.WriteString(w, "No change\n")
		return
	}
	io.WriteString(w, fmt.Sprintf("%s\n", sha1))
	w.Write(b)
}
