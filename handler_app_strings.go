package main

import (
	"fmt"
	"net/http"
	"sort"

	"code.google.com/p/gorilla/mux"
)

type ModelAppStrings struct {
	App            *App
	User           string
	CanDuplicate   bool
	StringsCount   int
	Strings        []string
	DeletedStrings []string
	RedirectUrl    string
}

// the only people that can duplicate translations
func canUserDuplicate(user string) bool {
	return user == "kjk" || user == "zeniko_ch"
}

func buildModelAppStrings(app *App, user string) *ModelAppStrings {
	langInfos := app.store.LangInfos()
	// it doesn't matter which language, they all have the same strings
	langInfo := langInfos[0]
	strings := make([]string, 0)
	for _, tr := range langInfo.Translations {
		strings = append(strings, tr.String)
	}
	sort.Strings(strings)
	model := &ModelAppStrings{
		App:            app,
		User:           user,
		CanDuplicate:   canUserDuplicate(user),
		Strings:        strings,
		DeletedStrings: app.store.GetDeletedStrings(),
		StringsCount:   len(strings),
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
}
