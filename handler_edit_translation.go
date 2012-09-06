package main

import (
	"fmt"
	"net/http"
	"strings"
)

// handler for url: /edittranslation
func handleEditTranslation(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	langCode := strings.TrimSpace(r.FormValue("lang"))
	if !IsValidLangCode(langCode) {
		serveErrorMsg(w, fmt.Sprintf("Invalid lang code '%s'", langCode))
		return
	}
	user := decodeUserFromCookie(r)
	if user == "" {
		serveErrorMsg(w, "User doesn't exist")
		return
	}
	str := strings.TrimSpace(r.FormValue("string"))
	translation := strings.TrimSpace(r.FormValue("translation"))
	//fmt.Printf("Adding translation: '%s'=>'%s', lang='%s'", str, translation, langCode)

	if err := app.translationLog.writeNewTranslation(str, translation, langCode, user); err != nil {
		serveErrorMsg(w, fmt.Sprintf("Failed to add a translation '%s'", err.Error()))
		return
	}
	// TODO: use a redirect with message passed in as an argument
	model := buildModelAppTranslations(app, langCode, decodeUserFromCookie(r))
	model.ShowTranslationEditedMsg = true
	if err := GetTemplates().ExecuteTemplate(w, tmplAppTrans, model); err != nil {
		fmt.Print(err.Error(), "\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
