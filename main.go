package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	// this is where we store, in an append-only fashion,
	// information about users and applications. All in
	// one place because I expect this information to be
	// very small
	dataFileName = "apptranslator.js"
)

type User struct {
	Login string
}

type App struct {
	Name      string
	AdminUser string // corresponds to User.Login field
	// a secret value that must be provided when uploading
	// new strings for translation
	UploadSecret string
}

type AppState struct {
	Users []*User
	Apps  []*App
}

var (
	appState        = AppState{}
	staticDir       = filepath.Join("www", "static")
	tmplMain        = "main.html"
	tmplMainPath    = filepath.Join("www", tmplMain)
	templates       = template.Must(template.ParseFiles(tmplMainPath))
	reloadTemplates = true
)

func GetTemplates() *template.Template {
	if reloadTemplates {
		return template.Must(template.ParseFiles(tmplMainPath))
	}
	return templates
}

func saveData() {
	b, err := json.Marshal(&appState)
	if err != nil {
		fmt.Printf("FileStorage.SetJSON, Marshal json failed (%v):%s\n", appState, err)
		return
	}
	ioutil.WriteFile(dataFileName, b, 0600)
}

func readDataAtStartup() error {
	b, err := ioutil.ReadFile(dataFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(b, &appState)
}

func serve404(w http.ResponseWriter) {
	fmt.Fprint(w, `<html><body>Page Not Found!</body></html>`)
}

func serveFileFromDir(w http.ResponseWriter, r *http.Request, dir, fileName string) {
	filePath := filepath.Join(dir, fileName)
	http.ServeFile(w, r, filePath)
	/*
		b, err := ioutil.ReadFile(filePath)
		if err != nil {
			fmt.Printf("serveFileFromDir() file=%s doesn't exist\n", filePath)
			serve404(w)
			return
		}
		w.Write(b)	
		fmt.Printf("serveFileFromDir() served %d bytes of '%s'\n", len(b), filePath)
	*/
}

func serveFileStatic(w http.ResponseWriter, r *http.Request, fileName string) {
	serveFileFromDir(w, r, staticDir, fileName)
}

const lenStatic = len("/static/")

// handler for url: /static/
func handleStatic(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[lenStatic:]
	serveFileStatic(w, r, file)
}

type TmplMainVars struct {
	Apps     *[]*App
	LoggedIn bool
	ErrorMsg string
}

// handler for url: /
func handleMain(w http.ResponseWriter, r *http.Request) {
	var errmsg string
	vars := &TmplMainVars{&appState.Apps, true, errmsg}
	err := GetTemplates().ExecuteTemplate(w, tmplMain, vars)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// handler for url: /addapp
// Form with values: appName and appUrl
func handleAddApp(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("appName"))
	appUrl := strings.TrimSpace(r.FormValue("appUrl"))
	var errmsg string
	if len(appName) == 0 || len(appUrl) == 0 {
		errmsg = "Must provide application name and url"
	}
	vars := &TmplMainVars{&appState.Apps, true, errmsg}
	err := GetTemplates().ExecuteTemplate(w, tmplMain, vars)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleApp(w http.ResponseWriter, r *http.Request) {
	// TODO: write me!
}

func main() {
	appState.Apps = append(appState.Apps, &App{"SumatraPDF", "kjk", "secret"})
	if err := readDataAtStartup(); err != nil {
		fmt.Printf("Failed to open data file %s. Can't proceed.\n", dataFileName)
		return
	}
	fmt.Printf("Read the data from %s\n", dataFileName)

	http.HandleFunc("/static/", handleStatic)
	http.HandleFunc("/addapp", handleAddApp)
	http.HandleFunc("/app/", handleApp)
	http.HandleFunc("/", handleMain)

	port := ":8890"
	fmt.Printf("Running on %s\n", port)
	http.ListenAndServe(port, nil)
	fmt.Printf("Exited\n")
}
