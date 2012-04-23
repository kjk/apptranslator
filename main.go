package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// this is where we store, in an append-only fashion,
	// information about users and applications. All in
	// one place because I expect this information to be
	// very small
	dataDir      = "data"
	dataFileName = "apptranslator.js"
)

type User struct {
	Login string
}

type TranslationsForString struct {
	LangCodes    []string
	Translations []string
}

func NewTranslationsForString() *TranslationsForString {
	return &TranslationsForString{make([]string, 0), make([]string, 0)}
}

func (t *TranslationsForString) Replace(langCode, trans string) {
	for i, v := range t.LangCodes {
		if v == langCode {
			t.Translations[i] = trans
			return
		}
	}
	t.LangCodes = append(t.LangCodes, langCode)
	t.Translations = append(t.Translations, trans)
}

type App struct {
	Name      string
	Url       string
	AdminUser string // corresponds to User.Login field
	// a secret value that must be provided when uploading
	// new strings for translation
	UploadSecret string
	// we don't want to serialize translations
	translations map[string]*TranslationsForString

	// used in templates
	LangsCount        int
	StringsCount      int
	UntranslatedCount int
}

type LogTranslationChange struct {
	LangCode       string
	EnglishStr     string
	NewTranslation string
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
	tmplApp         = "app.html"
	tmplAppPath     = filepath.Join("www", tmplApp)
	templates       = template.Must(template.ParseFiles(tmplMainPath, tmplAppPath))
	reloadTemplates = true
)

func GetTemplates() *template.Template {
	if reloadTemplates {
		return template.Must(template.ParseFiles(tmplMainPath, tmplAppPath))
	}
	return templates
}

func saveData() {
	b, err := json.Marshal(&appState)
	if err != nil {
		fmt.Printf("FileStorage.SetJSON, Marshal json failed (%v):%s\n", appState, err)
		return
	}
	path := filepath.Join(dataDir, dataFileName)
	ioutil.WriteFile(path, b, 0600)
}

func buildAppData(app *App) {
	fmt.Printf("buildAppData(): %s\n", app.Name)
	langs := make(map[string]bool)
	app.StringsCount = len(app.translations)
	app.UntranslatedCount = 0
	for _, t := range app.translations {
		for _, langCode := range t.LangCodes {
			langs[langCode] = true
		}
	}
	app.LangsCount = len(langs)
	for _, t := range app.translations {
		app.UntranslatedCount += app.LangsCount - len(t.LangCodes)
	}
}

// we ignore errors when reading
func readTranslationsFromLog(app *App) {
	app.translations = make(map[string]*TranslationsForString)
	fileName := app.Name + "_trans.dat"
	path := filepath.Join(dataDir, fileName)
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("readTranslationsFromLog(): file '%s' doesn't exist\n", path)
		return
	}
	defer file.Close()
	decoder := gob.NewDecoder(file)
	var logentry LogTranslationChange
	entries := 0
	for {
		err := decoder.Decode(&logentry)
		if err != nil {
			fmt.Printf("Finished decoding, %s\n", err.Error())
			break
		}
		entries++
		s := logentry.EnglishStr
		trans, ok := app.translations[s]
		if !ok {
			trans = NewTranslationsForString()
			app.translations[s] = trans
		}
		trans.Replace(logentry.LangCode, logentry.NewTranslation)
	}
	fmt.Printf("Found %d translation log entries for app %s\n", entries, app.Name)
}

func readDataAtStartup() error {
	path := filepath.Join(dataDir, dataFileName)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	err = json.Unmarshal(b, &appState)
	if err != nil {
		fmt.Printf("readDataAtStartup(): error json decoding '%s', %s\n", path, err.Error())
		return err
	}
	for _, app := range appState.Apps {
		readTranslationsFromLog(app)
		buildAppData(app)
	}
	return nil
}

func serve404(w http.ResponseWriter) {
	fmt.Fprint(w, `<html><body>Page Not Found!</body></html>`)
}

func serverErrorMsg(w http.ResponseWriter, msg string) {
	fmt.Fprintf(w, `<html><body>Error: %s</body></html>`, msg)
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

type ModelMain struct {
	Apps     *[]*App
	LoggedIn bool
	ErrorMsg string
}

// handler for url: /
func handleMain(w http.ResponseWriter, r *http.Request) {
	model := &ModelMain{&appState.Apps, true, ""}
	if err := GetTemplates().ExecuteTemplate(w, tmplMain, model); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func appAlreadyExists(appName string) bool {
	for _, app := range appState.Apps {
		if app.Name == appName {
			return true
		}
	}
	return false
}

func genAppUploadSecret() string {
	t := time.Now()
	rand.Seed(t.UnixNano())
	count := 8
	maxLetter := int('z' - 'a')
	r := make([]byte, count)
	for i := 0; i < count; i++ {
		letter := rand.Intn(maxLetter)
		r[i] = byte('a' + letter)
	}
	return string(r)
}

func addApp(app *App) {
	appState.Apps = append(appState.Apps, app)
	saveData()
}

// handler for url: /addapp
// Form with values: appName and appUrl
func handleAddApp(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("appName"))
	appUrl := strings.TrimSpace(r.FormValue("appUrl"))
	var errmsg string
	if len(appName) == 0 || len(appUrl) == 0 {
		errmsg = "Must provide application name and url"
	} else if appAlreadyExists(appName) {
		errmsg = fmt.Sprintf("Application %s already exists. Choose a different name.", appName)
	}
	if errmsg == "" {
		app := &App{appName, appUrl, "dummy", genAppUploadSecret(), nil, 0, 0, 0}
		addApp(app)

	}
	model := &ModelMain{&appState.Apps, true, errmsg}
	if err := GetTemplates().ExecuteTemplate(w, tmplMain, model); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func findApp(name string) *App {
	for _, app := range appState.Apps {
		if app.Name == name {
			return app
		}
	}
	return nil
}

type LangInfo struct {
	Code              string
	Name              string
	Strings           []string
	Translations      []string
	Untranslated      []string
	UntranslatedCount int
}

func NewLangInfo(langCode string) *LangInfo {
	li := new(LangInfo)
	li.Code = langCode
	li.Name = LangNameByCode(langCode)
	li.Strings = make([]string, 0)
	li.Translations = make([]string, 0)
	li.Untranslated = make([]string, 0)
	return li
}

type ModelApp struct {
	App   *App
	Langs []*LangInfo
}

// so that we can sort ModelApp.Langs by name
type LangInfoSeq []*LangInfo

func (s LangInfoSeq) Len() int      { return len(s) }
func (s LangInfoSeq) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type ByName struct{ LangInfoSeq }

func (s ByName) Less(i, j int) bool { return s.LangInfoSeq[i].Name < s.LangInfoSeq[j].Name }

func buildModelApp(app *App) *ModelApp {
	model := new(ModelApp)
	model.App = app
	langs := make(map[string]*LangInfo)
	total := 0
	for str, transForString := range app.translations {
		total += len(transForString.LangCodes)
		for i, langCode := range transForString.LangCodes {
			translation := transForString.Translations[i]
			langInfo, ok := langs[langCode]
			if !ok {
				langInfo = NewLangInfo(langCode)
				langs[langCode] = langInfo
			}
			langInfo.Strings = append(langInfo.Strings, str)
			langInfo.Translations = append(langInfo.Translations, translation)
		}
	}
	model.Langs = make([]*LangInfo, 0)
	for _, lang := range langs {
		model.Langs = append(model.Langs, lang)
		// TODO: calculate untranslated for each lang
	}
	sort.Sort(ByName{model.Langs})
	return model
}

// handler for url: /app/?name=%s
func handleApp(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("name"))
	fmt.Printf("handleApp() appName=%s\n", appName)
	app := findApp(appName)
	if app == nil {
		serverErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
	}
	model := buildModelApp(app)
	if err := GetTemplates().ExecuteTemplate(w, tmplApp, model); err != nil {
		fmt.Print(err.Error(), "\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	if err := readDataAtStartup(); err != nil {
		fmt.Printf("Failed to open data file %s. Can't proceed.\n", dataFileName)
		return
	}
	fmt.Printf("Read the data from %s\n", dataFileName)
	// for testing, add a dummy app if no apps exist
	if len(appState.Apps) == 0 {
		addApp(&App{"SumatraPDF", "http://blog.kowalczyk.info", "kjk", genAppUploadSecret(), nil, 0, 0, 0})
		fmt.Printf("Added dummy SumatraPDF app")
	}

	http.HandleFunc("/static/", handleStatic)
	http.HandleFunc("/addapp", handleAddApp)
	http.HandleFunc("/app/", handleApp)
	http.HandleFunc("/", handleMain)

	port := ":8890"
	fmt.Printf("Running on %s\n", port)
	http.ListenAndServe(port, nil)
	fmt.Printf("Exited\n")
}
