package main

import (
	"fmt"
	"net/http"

	"code.google.com/p/gorilla/mux"
)

// url: /app/{appname}/strings
func handleAppStrings(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appName := vars["appname"]
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application \"%s\" doesn't exist", appName))
		return
	}

	serveErrorMsg(w, fmt.Sprintf("strings NYI, app: '%s'", appName))
	/*
		//fmt.Printf("handleAppTranslations() appName=%s, lang=%s\n", app.Name, lang)
		model := buildModelAppTranslations(app, lang, decodeUserFromCookie(r))
		model.RedirectUrl = r.URL.String()
		ExecTemplate(w, tmplAppTrans, model)
	*/
}
