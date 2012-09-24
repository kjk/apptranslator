// This code is under BSD license. See license-bsd.txt
package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type ModelApp struct {
	App          *App
	PageTitle    string
	Langs        []*LangInfo
	RecentEdits  []Edit
	Translators  []*Translator
	SortedByName bool
	LoggedUser   string
	UserIsAdmin  bool
	RedirectUrl  string
}

// for sorting by count of translations
type TranslatorsSeq []*Translator

func (s TranslatorsSeq) Len() int      { return len(s) }
func (s TranslatorsSeq) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type ByCount struct{ TranslatorsSeq }

func (s ByCount) Less(i, j int) bool {
	return s.TranslatorsSeq[i].TranslationsCount > s.TranslatorsSeq[j].TranslationsCount
}

func sortTranslatorsByCount(t []*Translator) {
	sort.Sort(ByCount{t})
}

func buildModelApp(app *App, loggedUser string, sortedByName bool) *ModelApp {
	model := &ModelApp{
		App:          app,
		LoggedUser:   loggedUser,
		SortedByName: sortedByName,
		UserIsAdmin:  userIsAdmin(app, loggedUser),
		PageTitle:    fmt.Sprintf("Translations for %s", app.Name),
		Langs:        app.translationLog.LangInfos(),
		RecentEdits:  app.translationLog.recentEdits(10),
		Translators:  app.translationLog.translators()}
	sortTranslatorsByCount(model.Translators)
	// by default they are sorted by untranslated count
	if sortedByName {
		sortLangsByName(model.Langs)
	}
	return model
}

// handler for url: /app/{appname}?sort=name
func handleApp(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appName := vars["appname"]
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application \"%s\" doesn't exist", appName))
		return
	}

	sortOrder := strings.TrimSpace(r.FormValue("sort"))
	sortedByName := sortOrder == "name"
	//fmt.Printf("handleApp() appName=%s\n", appName)
	model := buildModelApp(app, decodeUserFromCookie(r), sortedByName)
	model.SortedByName = sortedByName

	model.RedirectUrl = r.URL.String()
	if err := GetTemplates().ExecuteTemplate(w, tmplApp, model); err != nil {
		fmt.Print(err.Error(), "\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
