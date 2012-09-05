package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
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
