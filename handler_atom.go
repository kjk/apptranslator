package main

import (
	"atomfeedgen"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func getAllRss(app *App) string {
	title := fmt.Sprintf("Apptranslator %s edits", app.config.Name)
	link := fmt.Sprintf("http://apptranslator.org/app?name=%s", app.config.Name) // TODO: technically url-escape
	dsc := fmt.Sprintf("Recent translation edits for %s", app.config.Name)
	feed := &atomfeedgen.Feed{Title: title, Link: link, Description: dsc}
	items := make([]*atomfeedgen.FeedItem, 0)
	for _, e := range app.translationLog.recentEdits(10) {
		title = "Item 1"
		// TODO: needs to remember dates of entries to show them here
		pubdate := time.Now()
		// TODO: a unique link?
		dsc = fmt.Sprintf("%s translated %s as %s in language %s", e.User, e.Text, e.Translation, e.Lang)
		i := &atomfeedgen.FeedItem{Title: title, Link: link, Description: dsc, PubDate: pubdate}
		items = append(items, i)
	}
	feed.Items = items
	return atomfeedgen.GenerateAtomFeed(feed)
}

func getRss(app *App, lang string) string {
	if "" == lang {
		return getAllRss(app)
	}
	title := fmt.Sprintf("Apptranslator %s edits for lang %s", app.config.Name, lang)
	link := fmt.Sprintf("http://apptranslator.org/app?name=%s&lang=%s", app.config.Name, lang) // TODO: technically url-escape
	dsc := fmt.Sprintf("Recent %s translation edits for %s", lang, app.config.Name)
	feed := &atomfeedgen.Feed{Title: title, Link: link, Description: dsc}
	return atomfeedgen.GenerateAtomFeed(feed)
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
