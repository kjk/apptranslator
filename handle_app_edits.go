package main

import (
	"net/http"

	"github.com/gorilla/mux"
)

// url: /app/{appname}/edits
func handleAppEdits(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appName := vars["appname"]
	app := findApp(appName)
	if app == nil {
		httpErrorf(w, "Application %q doesn't exist", appName)
		return
	}

	httpErrorf(w, "edits NYI, app: %q", appName)
	/*
		//fmt.Printf("handleAppTranslations() appName=%s, lang=%s\n", app.Name, lang)
		model := buildModelAppTranslations(app, lang, decodeUserFromCookie(r))
		model.RedirectUrl = r.URL.String()
		ExecTemplate(w, tmplAppTrans, model)
	*/
}
