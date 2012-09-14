package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"html/template"
	"net/http"
	"sort"
)

type ModelApp struct {
	App         *App
	Langs       []*LangInfo
	RecentEdits []Edit
	Translators []*Translator
	User        string // TODO: change to LoginName
	UserIsAdmin bool
	RedirectUrl string
}

// for sorting by name
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

func buildModelApp(app *App, user string) *ModelApp {
	model := &ModelApp{App: app, User: user, UserIsAdmin: userIsAdmin(app, user)}
	model.Langs = app.translationLog.LangInfos()
	model.RecentEdits = app.translationLog.recentEdits(10)
	model.Translators = app.translationLog.translators()
	sortTranslatorsByCount(model.Translators)
	return model
}

// handler for url: /app/{appname}
func handleApp(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appName := vars["appname"]
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application \"%s\" doesn't exist", appName))
		return
	}

	//fmt.Printf("handleApp() appName=%s\n", appName)
	model := buildModelApp(app, decodeUserFromCookie(r))
	model.RedirectUrl = r.URL.String()
	tp := &templateParser{}
	if err := GetTemplates().ExecuteTemplate(tp, tmplApp, model); err != nil {
		fmt.Print(err.Error(), "\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	content := &content{template.HTML(tp.HTML)}
	if err := GetTemplates().ExecuteTemplate(w, tmplBase, content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
