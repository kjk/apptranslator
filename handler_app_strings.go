package main

import (
	"fmt"
	"net/http"
	"sort"

	"code.google.com/p/gorilla/mux"
)

type ModelAppStrings struct {
	App          *App
	User         string
	StringsCount int
	Strings      []string
	RedirectUrl  string
}

func buildModelAppStrings(app *App, user string) *ModelAppStrings {
	langInfos := app.translationLog.LangInfos()
	// it doesn't matter which language, they all have the same strings
	langInfo := langInfos[0]
	strings := make([]string, 0)
	for _, tr := range langInfo.Translations {
		strings = append(strings, tr.String)
	}
	sort.Strings(strings)
	model := &ModelAppStrings{
		App:          app,
		User:         user,
		Strings:      strings,
		StringsCount: len(strings),
	}
	return model
}

// url: /app/{appname}/strings
func handleAppStrings(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appName := vars["appname"]
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application \"%s\" doesn't exist", appName))
		return
	}
	model := buildModelAppStrings(app, decodeUserFromCookie(r))
	model.RedirectUrl = r.URL.String()
	ExecTemplate(w, tmplAppStrings, model)
	//serveErrorMsg(w, fmt.Sprintf("strings NYI, app: '%s'", appName))
	/*
		//fmt.Printf("handleAppTranslations() appName=%s, lang=%s\n", app.Name, lang)
		model := buildModelAppTranslations(app, lang, decodeUserFromCookie(r))
		model.RedirectUrl = r.URL.String()
	*/
}
