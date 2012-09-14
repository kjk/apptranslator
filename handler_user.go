package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"html/template"
	"net/http"
)

type ModelUser struct {
	Name            string
	TranslatedCount int
	Edits           []*Edit
	User            string // TODO: change to LoginName
	RedirectUrl     string
}

func buildModelUser(user, loginName string) *ModelUser {
	model := &ModelUser{Name: user, User: loginName}
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
	tp := &templateParser{}
	if err := GetTemplates().ExecuteTemplate(tp, tmplUser, model); err != nil {
		fmt.Print(err.Error(), "\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	content := &content{template.HTML(tp.HTML)}
	if err := GetTemplates().ExecuteTemplate(w, tmplBase, content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
