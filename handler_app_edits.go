package main

import (
	"fmt"
	"net/http"

	"code.google.com/p/gorilla/mux"
)

// url: /app/{appname}/edits
func handleAppEdits(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appName := vars["appname"]
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application \"%s\" doesn't exist", appName))
		return
	}

	serveErrorMsg(w, fmt.Sprintf("edits NYI, app: %q", appName))
	/*
		//fmt.Printf("handleAppTranslations() appName=%s, lang=%s\n", app.Name, lang)
		model := buildModelAppTranslations(app, lang, decodeUserFromCookie(r))
		model.RedirectUrl = r.URL.String()
		ExecTemplate(w, tmplAppTrans, model)
	*/
}
