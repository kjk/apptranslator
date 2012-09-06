package main

import (
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
	model := &ModelAppTranslations{App: app, ShowTranslationEditedMsg: false, User: user, UserIsAdmin: userIsAdmin(app, user)}
	modelApp := buildModelApp(app, user)
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

// handler for url: /app?name=$app&lang=$lang
func handleAppTranslations(w http.ResponseWriter, r *http.Request, app *App, langCode string) {
	//fmt.Printf("handleAppTranslations() appName=%s, lang=%s\n", app.config.Name, langCode)
	model := buildModelAppTranslations(app, langCode, decodeUserFromCookie(r))
	model.RedirectUrl = r.URL.String()
	if err := GetTemplates().ExecuteTemplate(w, tmplAppTrans, model); err != nil {
		fmt.Print(err.Error(), "\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
