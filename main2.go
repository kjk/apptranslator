package main

import (
	"flag"
	"log"
	"net/http"
	"fmt"
)

var (
	configPath = flag.String("config", "secrets.json", "Path to configuration file")
)

type App struct {
	config *AppConfig

}

func NewApp(config *AppConfig) *App {
	app := &App{config: config}
	app.initData()
	return app
}

func (app *App) initData() {
	fmt.Printf("Created %s app\n", app.config.Name)
}

func readAppData(app *App) error {
	//TODO: write me
	//if err := readTranslationsFromLog(app); err != nil {
	//	return err
	//}
	return nil
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

	fmt.Printf("Running on %s\n", *httpAddr)
	if err := http.ListenAndServe(*httpAddr, nil); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err.Error())
	}
	fmt.Printf("Exited\n")

}