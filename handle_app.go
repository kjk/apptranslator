// This code is under BSD license. See license-bsd.txt
package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gorilla/mux"
	"github.com/kjk/apptranslator/store"
)

type EditDisplay struct {
	store.Edit
	TextDisplay string
}

type ModelApp struct {
	App          *App
	PageTitle    string
	Langs        []*store.LangInfo
	RecentEdits  []EditDisplay
	Translators  []*store.Translator
	SortedByName bool
	LoggedUser   string
	UserIsAdmin  bool
	RedirectUrl  string
}

// for sorting by count of translations
type TranslatorsSeq []*store.Translator

func (s TranslatorsSeq) Len() int      { return len(s) }
func (s TranslatorsSeq) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type ByCount struct{ TranslatorsSeq }

func (s ByCount) Less(i, j int) bool {
	return s.TranslatorsSeq[i].TranslationsCount > s.TranslatorsSeq[j].TranslationsCount
}

func sortTranslatorsByCount(t []*store.Translator) {
	sort.Sort(ByCount{t})
}

func strTruncate(s string, n int) string {
	if len(s) < n {
		return s
	}
	if len(s) < n+len("...") {
		return s
	}
	return s[:n] + "..."
}

func buildModelApp(app *App, loggedUser string, sortedByName bool) *ModelApp {
	edits := app.store.RecentEdits(10)
	editsDisplay := make([]EditDisplay, len(edits), len(edits))
	for i, e := range edits {
		ed := EditDisplay{Edit: e, TextDisplay: strTruncate(e.Text, 42)}
		editsDisplay[i] = ed
	}
	model := &ModelApp{
		App:          app,
		LoggedUser:   loggedUser,
		SortedByName: sortedByName,
		UserIsAdmin:  userIsAdmin(app, loggedUser),
		PageTitle:    fmt.Sprintf("Translations for %s", app.Name),
		Langs:        app.store.LangInfos(),
		RecentEdits:  editsDisplay,
		Translators:  app.store.Translators()}
	sortTranslatorsByCount(model.Translators)
	// by default they are sorted by untranslated count
	if sortedByName {
		store.SortLangsByName(model.Langs)
	}
	return model
}

// url: /app/{appname}?sort=name
func handleApp(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appName := vars["appname"]
	app := findApp(appName)
	if app == nil {
		httpErrorf(w, "Application %q doesn't exist", appName)
		return
	}

	sortOrder := strings.TrimSpace(r.FormValue("sort"))
	sortedByName := sortOrder == "name"
	//fmt.Printf("handleApp() appName=%s\n", appName)
	model := buildModelApp(app, decodeUserFromCookie(r), sortedByName)
	model.SortedByName = sortedByName

	model.RedirectUrl = r.URL.String()
	ExecTemplate(w, tmplApp, model)
}
