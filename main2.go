package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
)

var (
	configPath = flag.String("config", "secrets2.json", "Path to configuration file")
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
	/* lang := strings.TrimSpace(r.FormValue("lang"))
	if IsValidLangCode(lang) {
		handleAppTranslations(w, r, app, lang)
		return
	}*/

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
