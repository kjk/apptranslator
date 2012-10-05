// This code is under BSD license. See license-bsd.txt
package main

import (
	"atom"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func addEdits(edits []Edit, link string, feed *atom.Feed) {
	// technicall link should be unique for every entry and point to a page
	// for that entry, but we don't have pages for each string/translation
	for _, e := range edits {
		dsc := fmt.Sprintf("User '%s' translated '%s' as '%s' in language %s", e.User, e.Text, e.Translation, e.Lang)
		e := &atom.Entry{
			Title:       fmt.Sprintf("String: '%s'", e.Text),
			Link:        link,
			Description: dsc,
			PubDate:     e.Time}
		feed.AddEntry(e)
	}
}

func getAllRss(app *App) string {
	title := fmt.Sprintf("Apptranslator %s edits", app.Name)
	link := fmt.Sprintf("http://www.apptranslator.org/app/%s", app.Name) // TODO: url-escape app.Name

	edits := app.translationLog.recentEdits(10)
	pubTime := time.Now()
	if len(edits) > 0 {
		pubTime = edits[0].Time
	}
	feed := &atom.Feed{
		Title:   title,
		Link:    link,
		PubDate: pubTime,
	}
	addEdits(edits, link, feed)
	s, err := feed.GenXml()
	if err != nil {
		return "Failed to generate XML feed"
	}
	return s
}

func getRss(app *App, lang string) string {
	title := fmt.Sprintf("Apptranslator %s edits for language %s", app.Name, lang)
	// TODO: technically should url-escape
	link := fmt.Sprintf("http://apptranslator.org/app?name=%s&lang=%s", app.Name, lang)

	edits := app.translationLog.editsForLang(lang, 10)
	pubTime := time.Now()
	if len(edits) > 0 {
		pubTime = edits[0].Time
	}
	feed := &atom.Feed{
		Title:   title,
		Link:    link,
		PubDate: pubTime,
	}
	addEdits(edits, link, feed)
	s, err := feed.GenXml()
	if err != nil {
		return "Failed to generate XML feed"
	}
	return s
}

// url: /atom?app=$app&[lang=$lang]
func handleAtom(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application \"%s\" doesn't exist", appName))
		return
	}
	lang := strings.TrimSpace(r.FormValue("lang"))
	if 0 == len(lang) {
		// TODO: set headers to text/plain
		s := getAllRss(app)
		w.Write([]byte(s))
		return
	}

	if !IsValidLangCode(lang) {
		serveErrorMsg(w, fmt.Sprintf("Language \"%s\" is not valid", lang))
		return
	}

	w.Write([]byte(getRss(app, lang)))
	// TODO: set headers to text/plain
}
