// This code is under BSD license. See license-bsd.txt
package main

import (
	"fmt"
	"net/http"

	"code.google.com/p/gorilla/mux"
)

type EditByUser struct {
	Lang        string
	App         string
	Text        string
	Translation string
}

type ModelUser struct {
	Name            string
	PageTitle       string
	TranslatedCount int
	Edits           []EditByUser
	User            string // TODO: change to LoginName
	RedirectUrl     string
}

func buildModelUser(user, loginName string) *ModelUser {
	edits := make([]EditByUser, 0)
	for _, app := range appState.Apps {
		for _, edit := range app.store.EditsByUser(user) {
			var e = EditByUser{
				Lang:        edit.Lang,
				App:         app.Name,
				Text:        edit.Text,
				Translation: edit.Translation,
			}
			edits = append(edits, e)
		}
	}
	return &ModelUser{
		PageTitle: fmt.Sprintf("Translations by %s", user),
		Name:      user,
		User:      loginName,
		Edits:     edits,
	}
}

// url: /user/{user}
func handleUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userName := vars["user"]

	//fmt.Printf("handleUser() user=%s\n", userName)
	model := buildModelUser(userName, decodeUserFromCookie(r))
	model.RedirectUrl = r.URL.String()
	ExecTemplate(w, tmplUser, model)
}
