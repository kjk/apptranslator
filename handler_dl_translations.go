package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

type LangTrans struct {
	lang  string
	trans string
}

// handler for url: /downloadtranslations?name=$app
// Returns plain/text response in the format:
/*
AppTranslator: $appName
:string 1
cv:translation for cv language
pl:translation for pl language
:string 2
pl:translation for pl language
*/
// This format is designed for easy parsing
// TODO: allow for avoiding re-downloading of translations if they didn't
// change via some ETag-like or content hash functionality
func handleDownloadTranslations(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("name"))
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, fmt.Sprintf("AppTranslator: %s\n", app.config.Name))
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

	for s, ltarr := range m {
		io.WriteString(w, fmt.Sprintf(":%s\n", s))
		for _, lt := range ltarr {
			io.WriteString(w, fmt.Sprintf("%s:%s\n", lt.lang, lt.trans))
		}
	}
}
