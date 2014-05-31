// This code is under BSD license. See license-bsd.txt
package main

import (
	"fmt"
	"net/http"
	"strings"
)

// url: /duptranslation
func handleDuplicateTranslation(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	user := decodeUserFromCookie(r)
	if user == "" {
		serveErrorMsg(w, "User doesn't exist")
		return
	}
	if !canUserDuplicate(user) {
		serveErrorMsg(w, "User can't duplicate translations")
		return
	}
	str := strings.TrimSpace(r.FormValue("string"))
	duplicate := strings.TrimSpace(r.FormValue("duplicate"))
	fmt.Printf("Duplicating translation: '%s'=>'%s'\n", str, duplicate)

	if err := app.store.DuplicateTranslation(str, duplicate); err != nil {
		serveErrorMsg(w, fmt.Sprintf("Failed to duplicate translation '%s'", err))
		return
	}
	// TODO: use a redirect with message passed in as an argument
	model := buildModelAppStrings(app, decodeUserFromCookie(r))
	model.RedirectUrl = r.URL.String()
	ExecTemplate(w, tmplAppStrings, model)
}
