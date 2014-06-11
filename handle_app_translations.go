// This code is under BSD license. See license-bsd.txt
package main

import (
	"net/http"

	"github.com/kjk/apptranslator/store"

	"code.google.com/p/gorilla/mux"
)

type ModelAppTranslations struct {
	App                  *App
	LangInfo             *store.LangInfo
	User                 string
	UserIsAdmin          bool
	StringsCount         int
	TransProgressPercent int
	RedirectUrl          string
	Message              string
}

func buildModelAppTranslations(app *App, langCode, user string) *ModelAppTranslations {
	model := &ModelAppTranslations{
		App:         app,
		User:        user,
		UserIsAdmin: userIsAdmin(app, user)}

	modelApp := buildModelApp(app, user, false)
	for _, langInfo := range modelApp.Langs {
		if langInfo.Code != langCode {
			continue
		}
		model.LangInfo = langInfo
		model.StringsCount = len(langInfo.ActiveStrings)
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
	panic("buildModelAppTranslations() failed")
}

// url: /app/{appname}/{lang}?msg=${msg}
func handleAppTranslations(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	appName := vars["appname"]
	app := findApp(appName)
	if app == nil {
		httpErrorf(w, "Application %q doesn't exist", appName)
		return
	}

	langCode := vars["lang"]
	if !store.IsValidLangCode(langCode) {
		httpErrorf(w, "Invalid language: %q", langCode)
		return
	}
	msg := r.FormValue("msg")
	//fmt.Printf("handleAppTranslations() appName=%s, lang=%s\n", app.Name, langCode)
	model := buildModelAppTranslations(app, langCode, decodeUserFromCookie(r))
	model.Message = msg
	model.RedirectUrl = r.URL.String()
	ExecTemplate(w, tmplAppTrans, model)
}
