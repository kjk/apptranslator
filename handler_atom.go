package main

import (
	"atomfeed"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func getAllRss(app *App) string {
	title := fmt.Sprintf("Apptranslator %s edits", app.Name)
	link := fmt.Sprintf("http://www.apptranslator.org/app/%s", app.Name) // TODO: url-escape app.Name
	//dsc := fmt.Sprintf("Recent translation edits for %s", app.Name)

	// TODO: use a more reasonable time, like the time of last edit?
	pubTime, err := time.Parse(time.RFC3339, "2012-09-11T07:39:41Z")
	feed := &atomfeed.Feed{
		Title:   title,
		Link:    link,
		PubDate: pubTime}

	for _, e := range app.translationLog.recentEdits(10) {
		// TODO: better title
		title = "Item 1"
		// TODO: needs to remember dates of entries to show them here
		pubdate := time.Now()
		// TODO: a unique link?
		dsc := fmt.Sprintf("%s translated %s as %s in language %s", e.User, e.Text, e.Translation, e.Lang)
		e := &atomfeed.Entry{
			Title:       title,
			Link:        "http://blog.kowalczyk.info/item/1.html",
			Description: dsc,
			PubDate:     pubdate}
		feed.AddEntry(e)
	}
	s, err := feed.GenXml()
	if err != nil {
		return "Failed to generate XML feed"
	}
	return s
}

func getRss(app *App, lang string) string {
	if "" == lang {
		return getAllRss(app)
	}
	title := fmt.Sprintf("Apptranslator %s edits for lang %s", app.Name, lang)
	link := fmt.Sprintf("http://apptranslator.org/app?name=%s&lang=%s", app.Name, lang) // TODO: technically url-escape
	//dsc := fmt.Sprintf("Recent %s translation edits for %s", lang, app.Name)

	pubTime := time.Now() // TODO: better!
	feed := &atomfeed.Feed{
		Title:   title,
		Link:    link,
		PubDate: pubTime}

	s, err := feed.GenXml()
	if err != nil {
		return "Failed to generate XML feed"
	}
	return s
}

// handler for url: /atom?app=$app&[lang=$lang]
func handleAtom(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application \"%s\" doesn't exist", appName))
		return
	}
	lang := strings.TrimSpace(r.FormValue("lang"))

	if !IsValidLangCode(lang) {
		serveErrorMsg(w, fmt.Sprintf("Language \"%s\" is not valid", lang))
		return
	}

	//fmt.Printf("handleAtom() appName=%s\n", appName)
	w.Write([]byte(getRss(app, lang)))
	// TODO: set headers to text/plain
}
