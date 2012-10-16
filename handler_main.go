// This code is under BSD license. See license-bsd.txt
package main

import (
	"net/http"
)

type ModelMain struct {
	PageTitle   string
	Apps        *[]*App
	User        string
	UserIsAdmin bool
	ErrorMsg    string
	RedirectUrl string
}

// url: /
func handleMain(w http.ResponseWriter, r *http.Request) {
	if !isTopLevelUrl(r.URL.Path) {
		serve404(w, r)
		return
	}
	user := decodeUserFromCookie(r)
	model := &ModelMain{
		Apps:        &appState.Apps,
		User:        user,
		UserIsAdmin: false,
		RedirectUrl: r.URL.String(),
		PageTitle:   "AppTranslator - crowd-sourced translation for software"}

	ExecTemplate(w, tmplMain, model)
}
