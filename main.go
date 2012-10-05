// This code is under BSD license. See license-bsd.txt
package main

import (
	"code.google.com/p/gorilla/mux"
	"code.google.com/p/gorilla/securecookie"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/garyburd/go-oauth/oauth"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	configPath   = flag.String("config", "secrets.json", "Path to configuration file")
	httpAddr     = flag.String("addr", ":5000", "HTTP server address")
	logPath      = flag.String("log", "stdout", "where to log")
	inProduction = flag.Bool("production", false, "are we running in production")
	cookieName   = "ckie"
)

var (
	oauthClient = oauth.Client{
		TemporaryCredentialRequestURI: "https://api.twitter.com/oauth/request_token",
		ResourceOwnerAuthorizationURI: "https://api.twitter.com/oauth/authenticate",
		TokenRequestURI:               "https://api.twitter.com/oauth/access_token",
	}

	config = struct {
		TwitterOAuthCredentials *oauth.Credentials
		Apps                    []AppConfig
		CookieAuthKeyHexStr     *string
		CookieEncrKeyHexStr     *string
		AwsAccess               *string
		AwsSecret               *string
		S3BackupBucket          *string
		S3BackupDir             *string
	}{
		&oauthClient.Credentials,
		nil,
		nil, nil,
		nil, nil,
		nil, nil,
	}
	logger        *ServerLogger
	cookieAuthKey []byte
	cookieEncrKey []byte
	secureCookie  *securecookie.SecureCookie

	// this is where we store information about users and translation.
	// All in one place because I expect this data to be small
	dataDir string

	staticDir = "static"

	appState = AppState{}

	tmplMain      = "main.html"
	tmplApp       = "app.html"
	tmplAppTrans  = "apptrans.html"
	tmplUser      = "user.html"
	tmplLogs      = "logs.html"
	templateNames = [...]string{
		tmplMain, tmplApp, tmplAppTrans, tmplUser, tmplLogs, "header.html",
		"footer.html"}
	templatePaths   []string
	templates       *template.Template
	reloadTemplates = true
	alwaysLogTime   = true
)

func StringEmpty(s *string) bool {
	return s == nil || 0 == len(*s)
}

func S3BackupEnabled() bool {
	if StringEmpty(config.AwsAccess) {
		fmt.Print("Backup to s3 disabled because AwsAccess not defined in secrets.json\n")
		return false
	}
	if StringEmpty(config.AwsSecret) {
		fmt.Print("Backup to s3 disabled because AwsSecret not defined in secrets.json\n")
		return false
	}
	if StringEmpty(config.S3BackupBucket) {
		fmt.Print("Backup to s3 disabled because S3BackupBucket not defined in secrets.json\n")
		return false
	}
	if StringEmpty(config.S3BackupDir) {
		fmt.Print("Backup to s3 disabled because S3BackupDir not defined in secrets.json\n")
		return false
	}
	return true
}

// data dir is ../data on the server or ../apptranslatordata locally
func getDataDir() string {
	if dataDir != "" {
		return dataDir
	}
	dataDir = filepath.Join("..", "apptranslatordata")
	if PathExists(dataDir) {
		return dataDir
	}
	dataDir = filepath.Join("..", "..", "data")
	if PathExists(dataDir) {
		return dataDir
	}
	log.Fatal("data directory (../../data or ../apptranslatordata) doesn't exist")
	return ""
}

// a static configuration of a single app
type AppConfig struct {
	Name string
	// url for the application's website (shown in the UI)
	Url     string
	DataDir string
	// we authenticate only with Twitter, this is the twitter user name
	// of the admin user
	AdminTwitterUser string
	// an arbitrary string, used to protect the API for uploading new strings
	// for the app
	UploadSecret string
}

type User struct {
	Login string
}

type App struct {
	AppConfig
	translationLog *TranslationLog
}

type AppState struct {
	Users []*User
	Apps  []*App
}

func NewApp(config *AppConfig) *App {
	app := &App{AppConfig: *config}
	return app
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

func (a *App) translationLogFilePath() string {
	// the data directory and file 'translations.dat' must already
	// exists. We don't expect adding new projects often, it requires a
	// deploy anyway, so we force the admin to create those dirs
	appDataDir := filepath.Join(getDataDir(), a.DataDir)
	dataFilePath := filepath.Join(appDataDir, "translations.dat")
	if !PathExists(dataFilePath) {
		log.Fatalf("Data file %s for app %s doesn't exist. Prease create (empty file is ok)!\n", dataFilePath, a.Name)
	}
	return dataFilePath
}

func readAppData(app *App) error {
	l, err := NewTranslationLog(app.translationLogFilePath())
	if err != nil {
		return err
	}
	app.translationLog = l
	return nil
}

/*
type StringsSeq []string

func (s StringsSeq) Len() int      { return len(s) }
func (s StringsSeq) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type SmartString struct{ StringsSeq }

func (s SmartString) Less(i, j int) bool {
	return transStringLess(s.StringsSeq[i], s.StringsSeq[j])
}
*/

func findApp(name string) *App {
	for _, app := range appState.Apps {
		if app.Name == name {
			return app
		}
	}
	return nil
}

func appAlreadyExists(name string) bool {
	return nil != findApp(name)
}

func appInvalidField(app *App) string {
	app.Name = strings.TrimSpace(app.Name)
	if app.Name == "" {
		return "Name"
	}
	if app.DataDir == "" {
		return "DataDir"
	}
	if app.AdminTwitterUser == "" {
		return "AdminTwitterUser"
	}
	if app.UploadSecret == "" {
		return "UploadSecret"
	}
	return ""
}

func addApp(app *App) error {
	if invalidField := appInvalidField(app); invalidField != "" {
		return errors.New(fmt.Sprintf("App has invalid field '%s'", invalidField))
	}
	if appAlreadyExists(app.Name) {
		return errors.New("App already exists")
	}
	if err := readAppData(app); err != nil {
		return err
	}
	appState.Apps = append(appState.Apps, app)
	return nil
}

func GetTemplates() *template.Template {
	if reloadTemplates || (nil == templates) {
		if 0 == len(templatePaths) {
			for _, name := range templateNames {
				templatePaths = append(templatePaths, filepath.Join("tmpl", name))
			}
		}
		templates = template.Must(template.ParseFiles(templatePaths...))
	}
	return templates
}

func isTopLevelUrl(url string) bool {
	return 0 == len(url) || "/" == url
}

func serve404(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func serveErrorMsg(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
}

func userIsAdmin(app *App, user string) bool {
	return user == app.AdminTwitterUser
}

// readSecrets reads the configuration file from the path specified by
// the config command line flag.
func readSecrets(configFile string) error {
	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, &config)
	if err != nil {
		return err
	}
	cookieAuthKey, err = hex.DecodeString(*config.CookieAuthKeyHexStr)
	if err != nil {
		return err
	}
	cookieEncrKey, err = hex.DecodeString(*config.CookieEncrKeyHexStr)
	if err != nil {
		return err
	}
	secureCookie = securecookie.New(cookieAuthKey, cookieEncrKey)
	// verify auth/encr keys are correct
	val := map[string]string{
		"foo": "bar",
	}
	_, err = secureCookie.Encode(cookieName, val)
	if err != nil {
		// for convenience, if the auth/encr keys are not set,
		// generate valid, random value for them
		auth := securecookie.GenerateRandomKey(32)
		encr := securecookie.GenerateRandomKey(32)
		fmt.Printf("auth: %s\nencr: %s\n", hex.EncodeToString(auth), hex.EncodeToString(encr))
	}
	// TODO: somehow verify twitter creds
	return err
}

func makeTimingHandler(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		startTime := time.Now()
		fn(w, r)
		duration := time.Now().Sub(startTime)
		// log urls that take long time to generate i.e. over 1 sec in production
		// or over 0.1 sec in dev
		shouldLog := duration.Seconds() > 1.0
		if alwaysLogTime && duration.Seconds() > 0.1 {
			shouldLog = true
		}
		if shouldLog {
			url := r.URL.Path
			if len(r.URL.RawQuery) > 0 {
				url = fmt.Sprintf("%s?%s", url, r.URL.RawQuery)
			}
			logger.Noticef("'%s' took %f seconds to serve\n", url, duration.Seconds())
		}
	}
}

func main() {
	// set number of goroutines to number of cpus, but capped at 4 since
	// I don't expect this to be heavily trafficed website
	ncpu := runtime.NumCPU()
	if ncpu > 4 {
		ncpu = 4
	}
	runtime.GOMAXPROCS(ncpu)
	flag.Parse()

	if *inProduction {
		reloadTemplates = false
		alwaysLogTime = false
	}

	// TODO: pass *logPath to server logger
	logger = NewServerLogger(256, 256)

	/*
		if *logPath == "stdout" {
			logger = log.New(os.Stdout, "", 0)
		} else {
			loggerFile, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
			if err != nil {
				log.Fatalf("Failed to open log file '%s', %s\n", *logPath, err.Error())
			}
			defer loggerFile.Close()
			logger = log.New(loggerFile, "", 0)
		}*/

	if err := readSecrets(*configPath); err != nil {
		log.Fatalf("Failed reading config file %s. %s\n", *configPath, err.Error())
	}

	for _, appData := range config.Apps {
		app := NewApp(&appData)
		if err := addApp(app); err != nil {
			log.Fatalf("Failed to add the app: %s, err: %s\n", app.Name, err.Error())
		}
	}

	// for testing, add a dummy app if no apps exist
	if len(appState.Apps) == 0 {
		log.Fatalf("No apps defined in secrets.json")
	}

	r := mux.NewRouter()
	r.HandleFunc("/app/{appname}", makeTimingHandler(handleApp))
	r.HandleFunc("/app/{appname}/{lang}", makeTimingHandler(handleAppTranslations))
	r.HandleFunc("/user/{user}", makeTimingHandler(handleUser))
	r.HandleFunc("/edittranslation", makeTimingHandler(handleEditTranslation))
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

	backupConfig := &BackupConfig{
		AwsAccess: *config.AwsAccess,
		AwsSecret: *config.AwsSecret,
		Bucket:    *config.S3BackupBucket,
		S3Dir:     *config.S3BackupDir,
		LocalDir:  getDataDir(),
	}
	if S3BackupEnabled() {
		go BackupLoop(backupConfig)
	}
	msg := fmt.Sprintf("Started runing on %s", *httpAddr)
	logger.Noticef(msg)
	println(msg)
	if err := http.ListenAndServe(*httpAddr, nil); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err.Error())
	}
	fmt.Printf("Exited\n")
}
