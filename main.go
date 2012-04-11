package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

const (
	// this is where we store, in an append-only fashion,
	// information about users and projects. All in
	// one place because I expect this information to be
	// very small
	DataFileName = "apptranslator.js"
)

type User struct {
	Login string
}

type Project struct {
	Name      string
	AdminUser string // corresponds to User.Login field
	// a secret value that must be provided when uploading
	// new strings for translation
	UploadSecret string
}

type AppState struct {
	Users    []User
	Projects []Project
}

var (
	appState  = AppState{}
	staticDir = filepath.Join("www", "static")
)

func saveData() {
	b, err := json.Marshal(&appState)
	if err != nil {
		fmt.Printf("FileStorage.SetJSON, Marshal json failed (%v):%s\n", appState, err)
		return
	}
	ioutil.WriteFile(DataFileName, b, 0600)
}

func readDataAtStartup() error {
	b, err := ioutil.ReadFile(DataFileName)
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

func handleStatic(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[lenStatic:]
	serveFileStatic(w, r, file)
}

func handleMain(w http.ResponseWriter, r *http.Request) {
	serveFileFromDir(w, r, "www", "main.html")
}

func main() {
	if err := readDataAtStartup(); err != nil {
		fmt.Printf("Failed to open data file %s. Can't proceed.\n", DataFileName)
		return
	}
	fmt.Printf("Read the data from %s\n", DataFileName)

	http.HandleFunc("/static/", handleStatic)
	http.HandleFunc("/", handleMain)

	port := ":8888"
	fmt.Printf("Running on %s\n", port)
	http.ListenAndServe(port, nil)
}
