package main

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/kjk/apptranslator/store"
	"github.com/kjk/u"

	"code.google.com/p/gorilla/mux"
)

func serveFileFromDir(w http.ResponseWriter, r *http.Request, dir, fileName string) {
	filePath := filepath.Join(dir, fileName)
	if !u.PathExists(filePath) {
		fmt.Printf("serveFileFromDir() file=%s doesn't exist\n", filePath)
	}
	http.ServeFile(w, r, filePath)
}

func serveFileStatic(w http.ResponseWriter, r *http.Request, fileName string) {
	serveFileFromDir(w, r, staticDir, fileName)
}

const lenStatic = len("/s/")

// url: /s/
func handleStatic(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[lenStatic:]
	serveFileStatic(w, r, file)
}

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

// url: /edittranslation?string=${string}&translation=${translation}
func handleEditTranslation(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	app := findApp(appName)
	if app == nil {
		httpErrorf(w, "Application %q doesn't exist", appName)
		return
	}
	langCode := strings.TrimSpace(r.FormValue("lang"))
	if !store.IsValidLangCode(langCode) {
		httpErrorf(w, "Invalid lang code %q", langCode)
		return
	}
	user := decodeUserFromCookie(r)
	if user == "" {
		httpErrorf(w, "User doesn't exist")
		return
	}
	str := strings.TrimSpace(r.FormValue("string"))
	translation := r.FormValue("translation")

	if err := app.store.WriteNewTranslation(str, translation, langCode, user); err != nil {
		httpErrorf(w, "Failed to add a translation %q", err)
		return
	}
	msg := fmt.Sprintf("Edited translation of %q to be %q", str, translation)
	url := fmt.Sprintf("/app/%s/%s?msg=%s", appName, langCode, url.QueryEscape(msg))
	http.Redirect(w, r, url, http.StatusFound)
}

// url: /duptranslation?string=${string}&duplicate=${duplicate}lang=${langCode}
func handleDuplicateTranslation(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	langCode := strings.TrimSpace(r.FormValue("lang"))
	if !store.IsValidLangCode(langCode) {
		httpErrorf(w, "Invalid language: %q", langCode)
		return
	}

	app := findApp(appName)
	if app == nil {
		httpErrorf(w, "Application %q doesn't exist", appName)
		return
	}
	user := decodeUserFromCookie(r)
	if user == "" {
		httpErrorf(w, "User doesn't exist")
		return
	}
	if !userIsAdmin(app, user) {
		httpErrorf(w, "User can't duplicate translations")
		return
	}
	str := strings.TrimSpace(r.FormValue("string"))
	duplicate := strings.TrimSpace(r.FormValue("duplicate"))
	if str == duplicate {
		httpErrorf(w, "handleDuplicateTranslation: trying to duplicate the same string %q", str)
		return
	}

	if err := app.store.DuplicateTranslation(str, duplicate); err != nil {
		httpErrorf(w, "Failed to duplicate translation %q", err)
		return
	}

	msg := fmt.Sprintf("Duplicated %q as %q", str, duplicate)
	url := fmt.Sprintf("/app/%s/%s?msg=%s", appName, langCode, url.QueryEscape(msg))
	http.Redirect(w, r, url, http.StatusFound)
}

func InitHttpHandlers() {
	r := mux.NewRouter()
	r.HandleFunc("/app/{appname}", makeTimingHandler(handleApp))
	r.HandleFunc("/app/{appname}/edits", makeTimingHandler(handleAppEdits))
	r.HandleFunc("/app/{appname}/{lang}", makeTimingHandler(handleAppTranslations))
	r.HandleFunc("/user/{user}", makeTimingHandler(handleUser))
	r.HandleFunc("/edittranslation", makeTimingHandler(handleEditTranslation))
	r.HandleFunc("/duptranslation", makeTimingHandler(handleDuplicateTranslation))
	r.HandleFunc("/dltrans", makeTimingHandler(handleDownloadTranslations))
	r.HandleFunc("/uploadstrings", makeTimingHandler(handleUploadStrings))
	r.HandleFunc("/rss", makeTimingHandler(handleRss))

	r.HandleFunc("/login", handleLogin)
	r.HandleFunc("/oauthtwittercb", handleOauthTwitterCallback)
	r.HandleFunc("/logout", handleLogout)
	r.HandleFunc("/logs", makeTimingHandler(handleLogs))
	r.HandleFunc("/", makeTimingHandler(handleMain))

	http.HandleFunc("/s/", makeTimingHandler(handleStatic))
	http.Handle("/", r)
}
