// This code is under BSD license. See license-bsd.txt
package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/kjk/apptranslator/store"
)

// url: /duptranslation?string=${string}&duplicate=${duplicate}lang=${langCode}
func handleDuplicateTranslation(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	langCode := strings.TrimSpace(r.FormValue("lang"))
	if !store.IsValidLangCode(langCode) {
		serveErrorMsg(w, fmt.Sprintf("Invalid language: %q", langCode))
		return
	}

	if !store.IsValidLangCode(langCode) {
		serveErrorMsg(w, fmt.Sprintf("Application %q doesn't exist", appName))
		return
	}
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application %q doesn't exist", appName))
		return
	}
	user := decodeUserFromCookie(r)
	if user == "" {
		serveErrorMsg(w, "User doesn't exist")
		return
	}
	if !userIsAdmin(app, user) {
		serveErrorMsg(w, "User can't duplicate translations")
		return
	}
	str := strings.TrimSpace(r.FormValue("string"))
	duplicate := strings.TrimSpace(r.FormValue("duplicate"))
	if str == duplicate {
		serveErrorMsg(w, fmt.Sprintf("handleDuplicateTrnslation: trying to duplicate the same string %q", str))
	}

	if err := app.store.DuplicateTranslation(str, duplicate); err != nil {
		serveErrorMsg(w, fmt.Sprintf("Failed to duplicate translation %q", err))
		return
	}
	// TODO: use a redirect with message passed in as an argument
	model := buildModelAppTranslations(app, langCode, decodeUserFromCookie(r))
	model.RedirectUrl = r.URL.String()
	ExecTemplate(w, tmplAppTrans, model)
}
