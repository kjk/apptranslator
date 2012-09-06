package main

import (
	"bufio"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
)

var (
	configPath = flag.String("config", "secrets.json", "Path to configuration file")
)

type App struct {
	config         *AppConfig
	translationLog *TranslationLog
}

func NewApp(config *AppConfig) *App {
	app := &App{config: config}
	fmt.Printf("Created %s app\n", app.config.Name)
	return app
}

func readAppData(app *App) error {
	l, err := NewTranslationLog(app.translationLogFilePath())
	if err != nil {
		return err
	}
	app.translationLog = l
	return nil
}

// used in templates
func (a *App) LangsCount() int {
	return a.translationLog.LangsCount()
}

// used in templates
func (a *App) StringsCount() int {
	return a.translationLog.StringsCount()
}

// used in templates
func (a *App) UntranslatedCount() int {
	return a.translationLog.UntranslatedCount()
}

type StringsSeq []string

func (s StringsSeq) Len() int      { return len(s) }
func (s StringsSeq) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type SmartString struct{ StringsSeq }

func (s SmartString) Less(i, j int) bool {
	return transStringLess(s.StringsSeq[i], s.StringsSeq[j])
}

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

type ModelApp struct {
	App         *App
	Langs       []*LangInfo
	User        string
	UserIsAdmin bool
	RedirectUrl string
}

func buildModelApp(app *App, user string) *ModelApp {
	model := &ModelApp{App: app, User: user, UserIsAdmin: userIsAdmin(app, user)}
	model.Langs = app.translationLog.LangInfos()
	return model
}

// handler for url: /app?name=$app
func handleApp(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("name"))
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application \"%s\" doesn't exist", appName))
		return
	}
	lang := strings.TrimSpace(r.FormValue("lang"))
	if IsValidLangCode(lang) {
		handleAppTranslations(w, r, app, lang)
		return
	}

	//fmt.Printf("handleApp() appName=%s\n", appName)
	model := buildModelApp(app, decodeUserFromCookie(r))
	model.RedirectUrl = r.URL.String()
	tp := &templateParser{}
	if err := GetTemplates().ExecuteTemplate(tp, tmplApp, model); err != nil {
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

// handler for url: /edittranslation
func handleEditTranslation(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	langCode := strings.TrimSpace(r.FormValue("lang"))
	if !IsValidLangCode(langCode) {
		serveErrorMsg(w, fmt.Sprintf("Invalid lang code '%s'", langCode))
		return
	}
	user := decodeUserFromCookie(r)
	if user == "" {
		serveErrorMsg(w, "User doesn't exist")
		return
	}
	str := strings.TrimSpace(r.FormValue("string"))
	translation := strings.TrimSpace(r.FormValue("translation"))
	//fmt.Printf("Adding translation: '%s'=>'%s', lang='%s'", str, translation, langCode)

	if err := app.translationLog.writeNewTranslation(str, translation, langCode, user); err != nil {
		serveErrorMsg(w, fmt.Sprintf("Failed to add a translation '%s'", err.Error()))
		return
	}
	// TODO: use a redirect with message passed in as an argument
	model := buildModelAppTranslations(app, langCode, decodeUserFromCookie(r))
	model.ShowTranslationEditedMsg = true
	if err := GetTemplates().ExecuteTemplate(w, tmplAppTrans, model); err != nil {
		fmt.Print(err.Error(), "\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type LangTrans struct {
	lang  string
	trans string
}

// handler for url: /downloadtranslations?name=$app
// Returns plain/text response in the format:
/*
AppTranslator: $appName
:string 1
cv:translation for cv language
pl:translation for pl language
:string 2
pl:translation for pl language
*/
// This format is designed for easy parsing
// TODO: allow for avoiding re-downloading of translations if they didn't
// change via some ETag-like or content hash functionality
func handleDownloadTranslations(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("name"))
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, fmt.Sprintf("AppTranslator: %s\n", app.config.Name))
	m := make(map[string][]LangTrans)
	langInfos := app.translationLog.LangInfos()
	for _, li := range langInfos {
		code := li.Code
		for _, t := range li.Translations {
			if "" == t.Current() {
				continue
			}
			s := t.String
			l, exists := m[s]
			if !exists {
				l = make([]LangTrans, 0)
			}
			var lt LangTrans
			lt.lang = code
			lt.trans = t.Current()
			l = append(l, lt)
			m[s] = l
		}
	}

	for s, ltarr := range m {
		io.WriteString(w, fmt.Sprintf(":%s\n", s))
		for _, lt := range ltarr {
			io.WriteString(w, fmt.Sprintf("%s:%s\n", lt.lang, lt.trans))
		}
	}
}

type CantParseError struct {
	Msg    string
	LineNo int
}

func (e *CantParseError) Error() string {
	return fmt.Sprintf("Error: %s on line %d", e.Msg, e.LineNo)
}

func myReadLine(r *bufio.Reader) (string, error) {
	line, isPrefix, err := r.ReadLine()
	if err != nil {
		return "", err
	}
	if isPrefix {
		return "", &CantParseError{"Line too long", -1}
	}
	return string(line), nil
}

// returns nil on error
func parseUploadedStrings(reader io.Reader) []string {
	r := bufio.NewReaderSize(reader, 4*1024)
	res := make([]string, 0)
	parsedHeader := false
	for {
		line, err := myReadLine(r)
		if err != nil {
			if err == io.EOF {
				if 0 == len(res) {
					return nil
				}
				return res
			}
			return nil
		}
		if !parsedHeader {
			// first line must be: AppTranslator strings
			if line != "AppTranslator strings" {
				fmt.Printf("parseUploadedStrings(): invalid first line: '%s'\n", line)
				return nil
			}
			parsedHeader = true
		} else {
			if 0 == len(line) {
				fmt.Printf("parseUploadedStrings(): encountered empty line\n")
				return nil
			}
			res = append(res, line)
		}
	}
	return res
}

// handler for url: POST /handleUploadStrings?app=$appName&secret=$uploadSecret
// POST data is in the format:
/*
AppTranslator strings
string to translate 1
string to translate 2
...
*/
func handleUploadStrings(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	secret := strings.TrimSpace(r.FormValue("secret"))
	if secret != app.config.UploadSecret {
		serveErrorMsg(w, fmt.Sprintf("Invalid secret for app '%s'", appName))
		return
	}
	file, _, err := r.FormFile("strings")
	if err != nil {
		serveErrorMsg(w, fmt.Sprintf("No file with strings %s", err.Error()))
		return
	}
	defer file.Close()
	newStrings := parseUploadedStrings(file)
	if nil == newStrings {
		serveErrorMsg(w, "Error parsing uploaded strings")
		return
	}
	w.Write([]byte("Ok\n"))
}

func main() {
	flag.Parse()

	if err := readSecrets(*configPath); err != nil {
		log.Fatalf("Failed reading config file %s. %s\n", *configPath, err.Error())
	}

	for _, appData := range config.Apps {
		app := NewApp(&appData)
		if err := addApp(app); err != nil {
			log.Fatalf("Failed to add the app: %s, err: %s\n", app.config.Name, err.Error())
		}
	}

	// for testing, add a dummy app if no apps exist
	if len(appState.Apps) == 0 {
		log.Fatalf("No apps defined in secrets.json")
	}

	http.HandleFunc("/s/", makeTimingHandler(handleStatic))
	http.HandleFunc("/app", makeTimingHandler(handleApp))
	http.HandleFunc("/edittranslation", makeTimingHandler(handleEditTranslation))
	http.HandleFunc("/downloadtranslations", makeTimingHandler(handleDownloadTranslations))
	http.HandleFunc("/uploadstrings", makeTimingHandler(handleUploadStrings))

	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/oauthtwittercb", handleOauthTwitterCallback)
	http.HandleFunc("/logout", handleLogout)
	http.HandleFunc("/", makeTimingHandler(handleMain))

	fmt.Printf("Running on %s\n", *httpAddr)
	if err := http.ListenAndServe(*httpAddr, nil); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err.Error())
	}
	fmt.Printf("Exited\n")

}
