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

type Translation struct {
	String string
	// last string is current translation, previous strings
	// are a history of how translation changed
	Translations []string
}

func NewTranslation(s, trans string) *Translation {
	t := &Translation{String: s}
	t.Translations = make([]string, 0)
	if trans != "" {
		t.Translations = append(t.Translations, trans)
	}
	return t
}

func (t *Translation) Current() string {
	if 0 == len(t.Translations) {
		return ""
	}
	return t.Translations[len(t.Translations)-1]
}

func (t *Translation) updateTranslation(trans string) {
	t.Translations = append(t.Translations, trans)
}

type LangInfo struct {
	Code         string
	Name         string
	Translations []*Translation

	untranslatedCount int
}

func NewLangInfo(langCode string) *LangInfo {
	li := &LangInfo{Code: langCode, Name: LangNameByCode(langCode)}
	li.Translations = make([]*Translation, 0)
	return li
}

func (li *LangInfo) updateUntranslatedCount() {
	n := 0
	for _, t := range li.Translations {
		if len(t.Translations) == 0 {
			n++
		}
	}
	li.untranslatedCount = n
}

func (li *LangInfo) UntranslatedCount() int {
	return li.untranslatedCount
}

func (li *LangInfo) appendTranslation(str, trans string) {
	t := NewTranslation(str, trans)
	li.Translations = append(li.Translations, t)
	li.updateUntranslatedCount()
}

func (li *LangInfo) addTranslation(str, translation string) {
	for _, t := range li.Translations {
		if str == t.String {
			t.updateTranslation(translation)
			return
		}
	}
	li.appendTranslation(str, translation)
	li.updateUntranslatedCount()
}

func (li *LangInfo) addUntranslatedIfNotExists(str string) {
	for _, t := range li.Translations {
		if str == t.String {
			return
		}
	}
	li.appendTranslation(str, "")
}

type App struct {
	Name      string
	Url       string
	AdminUser string // corresponds to User.Login field
	// a secret value that must be provided when uploading
	// new strings for translation
	UploadSecret string

	langInfos  []*LangInfo
	allStrings map[string]bool
}

func NewApp(name, url string) *App {
	app := &App{Name: name, Url: url}
	app.langInfos = make([]*LangInfo, 0)
	app.UploadSecret = genAppUploadSecret()
	app.allStrings = make(map[string]bool)
	return app
}

// used in templates
func (a *App) LangsCount() int {
	return len(a.langInfos)
}

// used in templates
func (a *App) StringsCount() int {
	if len(a.langInfos) == 0 {
		return 0
	}
	return len(a.langInfos[0].Translations)
}

// used in templates
func (a *App) UntranslatedCount() int {
	n := 0
	for _, langInfo := range a.langInfos {
		n += langInfo.UntranslatedCount()
	}
	return n
}

func (a *App) langInfoByCode(langCode string, create bool) *LangInfo {
	for _, li := range a.langInfos {
		if li.Code == langCode {
			return li
		}
	}
	if create {
		li := NewLangInfo(langCode)
		a.langInfos = append(a.langInfos, li)
		return li
	}
	return nil
}

func (a *App) addUntranslated() {
	for s, _ := range a.allStrings {
		for _, li := range a.langInfos {
			li.addUntranslatedIfNotExists(s)
		}
	}
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
	appState  = AppState{}
	staticDir = filepath.Join("www", "static")

	tmplMain        = "main.html"
	tmplApp         = "app.html"
	tmplAppTrans    = "apptrans.html"
	templateNames   = [...]string{tmplMain, tmplApp, tmplAppTrans}
	templatePaths   = make([]string, 0)
	templates       *template.Template
	reloadTemplates = true
)

func GetTemplates() *template.Template {
	if reloadTemplates || (nil == templates) {
		if 0 == len(templatePaths) {
			for _, name := range templateNames {
				templatePaths = append(templatePaths, filepath.Join("www", name))
			}
		}
		templates = template.Must(template.ParseFiles(templatePaths...))
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
}

// we ignore errors when reading
func readTranslationsFromLog(app *App) {
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
		app.addTranslation(logentry.LangCode, logentry.EnglishStr, logentry.NewTranslation)
	}
	app.addUntranslated()
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
		if nil == app.allStrings {
			app.allStrings = make(map[string]bool)
		}
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
		app := NewApp(appName, appUrl)
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

const (
	stringCmpRemoveSet = ";,:()[]&_ "
)

func transStringLess(s1, s2 string) bool {
	s1 = strings.Trim(s1, stringCmpRemoveSet)
	s2 = strings.Trim(s2, stringCmpRemoveSet)
	s1 = strings.ToLower(s1)
	s2 = strings.ToLower(s2)
	return s1 < s2
}

type TranslationSeq []*Translation

func (s TranslationSeq) Len() int      { return len(s) }
func (s TranslationSeq) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type ByString struct{ TranslationSeq }

func (s ByString) Less(i, j int) bool {
	s1 := s.TranslationSeq[i].String
	s2 := s.TranslationSeq[j].String
	trans1 := s.TranslationSeq[i].Current()
	trans2 := s.TranslationSeq[j].Current()
	if trans1 == "" && trans2 != "" {
		return true
	}
	if trans2 == "" && trans1 != "" {
		return false
	}
	return transStringLess(s1, s2)
}

type StringsSeq []string

func (s StringsSeq) Len() int      { return len(s) }
func (s StringsSeq) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type SmartString struct{ StringsSeq }

func (s SmartString) Less(i, j int) bool {
	return transStringLess(s.StringsSeq[i], s.StringsSeq[j])
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

type ByUntranslated struct{ LangInfoSeq }

func (s ByUntranslated) Less(i, j int) bool {
	l1 := s.LangInfoSeq[i].UntranslatedCount()
	l2 := s.LangInfoSeq[j].UntranslatedCount()
	if l1 != l2 {
		return l1 > l2
	}
	// to make sort more stable, we compare by name if counts are the same
	return s.LangInfoSeq[i].Name < s.LangInfoSeq[j].Name
}

func calcUntranslated(app *App, langInfo *LangInfo) {
}

func buildModelApp(app *App) *ModelApp {
	model := new(ModelApp)
	model.App = app
	// could use App.langInfos directly but it's not
	// thread safe and really should sort after update
	// not on every read
	model.Langs = make([]*LangInfo, 0)
	for _, li := range app.langInfos {
		// TODO: sort on insert
		sort.Sort(ByString{li.Translations})
		model.Langs = append(model.Langs, li)
	}
	sort.Sort(ByUntranslated{model.Langs})
	return model
}

type ModelAppTranslations struct {
	App          *App
	LangInfo     *LangInfo
	StringsCount int
}

func buildModelAppTranslations(app *App, langCode string) *ModelAppTranslations {
	model := &ModelAppTranslations{App: app}
	modelApp := buildModelApp(app)
	for _, langInfo := range modelApp.Langs {
		if langInfo.Code == langCode {
			model.LangInfo = langInfo
			model.StringsCount = len(langInfo.Translations)
			return model
		}
	}
	panic("buildModelAppTranslations() failed")
}

// handler for url: /app/?name=$app&lang=$lang
func handleAppTranslations(w http.ResponseWriter, r *http.Request, app *App, langCode string) {
	fmt.Printf("handleAppTranslations() appName=%s, lang=%s\n", app.Name, langCode)
	model := buildModelAppTranslations(app, langCode)
	if err := GetTemplates().ExecuteTemplate(w, tmplAppTrans, model); err != nil {
		fmt.Print(err.Error(), "\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// handler for url: /app/?name=$app
func handleApp(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("name"))
	app := findApp(appName)
	if app == nil {
		serverErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	lang := strings.TrimSpace(r.FormValue("lang"))
	if IsValidLangCode(lang) {
		handleAppTranslations(w, r, app, lang)
		return
	}
	fmt.Printf("handleApp() appName=%s\n", appName)
	model := buildModelApp(app)
	if err := GetTemplates().ExecuteTemplate(w, tmplApp, model); err != nil {
		fmt.Print(err.Error(), "\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// handler for url: /edittranslation
func handleEditTranslation(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	app := findApp(appName)
	if app == nil {
		serverErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	langCode := strings.TrimSpace(r.FormValue("langCode"))
	if IsValidLangCode(langCode) {
		serverErrorMsg(w, fmt.Sprintf("Invalid lang code '%s'", langCode))
		return
	}
	str := strings.TrimSpace(r.FormValue("string"))
	translation := strings.TrimSpace(r.FormValue("translation"))
	app.addTranslation(langCode, str, translation)
	// TODO: url-escape app.Name and langCode
	url := fmt.Sprintf("/app/?name=%s&lang=%s", app.Name, langCode)
	http.Redirect(w, r, url, 301)

}

func (app *App) addTranslation(langCode, str, trans string) {
	li := app.langInfoByCode(langCode, true)
	li.addTranslation(str, trans)
	if _, ok := app.allStrings[str]; ok {
		return
	}
	app.allStrings[str] = true
}

func main() {
	if err := readDataAtStartup(); err != nil {
		fmt.Printf("Failed to open data file %s. Can't proceed.\n", dataFileName)
		return
	}
	fmt.Printf("Read the data from %s\n", dataFileName)
	// for testing, add a dummy app if no apps exist
	if len(appState.Apps) == 0 {
		app := NewApp("SumatraPDF", "http://blog.kowalczyk.info")
		addApp(app)
		fmt.Printf("Added dummy SumatraPDF app")
	}

	http.HandleFunc("/static/", handleStatic)
	http.HandleFunc("/addapp", handleAddApp)
	http.HandleFunc("/app/", handleApp)
	http.HandleFunc("/edittranslation", handleEditTranslation)
	http.HandleFunc("/", handleMain)

	port := ":8890"
	fmt.Printf("Running on %s\n", port)
	http.ListenAndServe(port, nil)
	fmt.Printf("Exited\n")
}
