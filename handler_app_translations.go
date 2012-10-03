// This code is under BSD license. See license-bsd.txt
package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"net/http"
)

type ModelAppTranslations struct {
	App                      *App
	LangInfo                 *LangInfo
	User                     string
	UserIsAdmin              bool
	StringsCount             int
	TransProgressPercent     int
	ShowTranslationEditedMsg bool
	RedirectUrl              string
}

func buildModelAppTranslations(app *App, langCode, user string) *ModelAppTranslations {
	model := &ModelAppTranslations{
		App: app,
		ShowTranslationEditedMsg: false,
		User:                     user,
		UserIsAdmin:              userIsAdmin(app, user)}

	modelApp := buildModelApp(app, user, false)
	for _, langInfo := range modelApp.Langs {
		if langInfo.Code == langCode {
			model.LangInfo = langInfo
			model.StringsCount = len(langInfo.Translations)
			if 0 == model.StringsCount {
				model.TransProgressPercent = 100
			} else {
				total := float32(model.StringsCount)
				translated := float32(model.StringsCount - langInfo.UntranslatedCount())
				perc := (100. * translated) / total
				model.TransProgressPercent = int(perc)
			}
			return model
		}
	}
	panic("buildModelAppTranslations() failed")
}

// url: /app/{appname}/{lang}
func handleAppTranslations(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appName := vars["appname"]
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application \"%s\" doesn't exist", appName))
		return
	}

	lang := vars["lang"]
	if !IsValidLangCode(lang) {
		serveErrorMsg(w, fmt.Sprintf("Invalid language: '%s'", lang))
		return
	}

	//fmt.Printf("handleAppTranslations() appName=%s, lang=%s\n", app.Name, lang)
	model := buildModelAppTranslations(app, lang, decodeUserFromCookie(r))
	model.RedirectUrl = r.URL.String()
	if err := GetTemplates().ExecuteTemplate(w, tmplAppTrans, model); err != nil {
		fmt.Print(err.Error(), "\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
