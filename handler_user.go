package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"net/http"
)

type ModelUser struct {
	Name            string
	PageTitle       string
	TranslatedCount int
	Edits           []*Edit
	User            string // TODO: change to LoginName
	RedirectUrl     string
}

func buildModelUser(user, loginName string) *ModelUser {
	model := &ModelUser{Name: user,
		User:      loginName,
		PageTitle: fmt.Sprintf("Translations of user %s", user)}
	// TOOD: calc Edits
	return model
}

// handler for url: /user/{user}
func handleUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userName := vars["user"]

	//fmt.Printf("handleUser() user=%s\n", userName)
	model := buildModelUser(userName, decodeUserFromCookie(r))
	model.RedirectUrl = r.URL.String()
	if err := GetTemplates().ExecuteTemplate(w, tmplUser, model); err != nil {
		fmt.Print(err.Error(), "\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
