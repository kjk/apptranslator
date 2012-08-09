package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"gorilla/securecookie"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"oauth"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	cookieName = "ckie"
)

var (
	oauthClient = oauth.Client{
		TemporaryCredentialRequestURI: "https://api.twitter.com/oauth/request_token",
		ResourceOwnerAuthorizationURI: "https://api.twitter.com/oauth/authenticate",
		TokenRequestURI:               "https://api.twitter.com/oauth/access_token",
	}

	config = struct {
		TwitterOAuthCredentials *oauth.Credentials
		App                     *AppConfig
		CookieAuthKeyHexStr     *string
		CookieEncrKeyHexStr     *string
	}{
		&oauthClient.Credentials,
		nil,
		nil,
		nil,
	}

	cookieAuthKey []byte
	cookieEncrKey []byte
	secureCookie  *securecookie.SecureCookie

	// this is where we store information about users and translation.
	// All in one place because I expect this data to be small
	dataDir string

	configPath = flag.String("config", "secrets.json", "Path to configuration file")
	httpAddr   = flag.String("addr", ":8089", "HTTP server address")
)

// data dir is ../data on the server or ../apptranslatordata locally
func getDataDir() string {
	if dataDir != "" {
		return dataDir
	}
	dataDir = filepath.Join("..", "apptranslatordata")
	if FileExists(dataDir) {
		return dataDir
	}
	dataDir = filepath.Join("..", "data")
	if FileExists(dataDir) {
		return dataDir
	}
	log.Fatal("data directory (../data or ../apptranslatordata) doesn't exist")
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
			li.updateUntranslatedCount()
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
	config *AppConfig

	// protects writing translations
	mu         sync.Mutex
	langInfos  []*LangInfo
	allStrings map[string]bool
}

func NewApp(config *AppConfig) *App {
	app := &App{config: config}
	app.initData()
	return app
}

func (app *App) initData() {
	if nil == app.allStrings {
		app.allStrings = make(map[string]bool)
	}
	langsCount := len(Languages)
	if nil == app.langInfos {
		app.langInfos = make([]*LangInfo, langsCount, langsCount)
		for i, lang := range Languages {
			app.langInfos[i] = NewLangInfo(lang.Code)
		}
	}
	fmt.Printf("Created %s app with %d langs\n", app.config.Name, len(app.langInfos))
}

func (a *App) translationLogFilePath() string {
	return filepath.Join(filepath.Join(getDataDir(), a.config.DataDir), "translations.dat")
}

func (a *App) writeTranslationToLog(langCode, str, trans, user string) {
	//fmt.Printf("writeTranslationToLog %s:%s => %s\n", langCode, str, trans)
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.translationLogFilePath()
	file, err := os.OpenFile(p, os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("writeTranslationToLog(): failed to open file '%s', %s\n", p, err.Error())
		return
	}
	defer file.Close()

	/*_, err = file.Seek(0, 2)
	if err != nil {
		fmt.Printf("writeTranslationToLog(): seek failed %s\n", err.Error())
		return
	}*/
	var logentry LogTranslationChange
	logentry.LangCode = langCode
	logentry.EnglishStr = str
	logentry.NewTranslation = trans
	logentry.User = user
	encoder := json.NewEncoder(file)
	// TODO: handle an error in some way (is there anything we can do)?
	err = encoder.Encode(logentry)
	if err != nil {
		fmt.Printf("writeTranslationToLog(): encoder.Encode() failed %s\n", err.Error())
		return
	}
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

func (app *App) addTranslation(langCode, str, trans string, allowNew bool) {
	obsolete, exists := app.allStrings[str]
	if !exists {
		if !allowNew {
			fmt.Printf("addTranslation(): skipped %s because not allowing adding strings\n", str)
			return
		}
		// if adding new, it's not obsolete, otherwise we preserve the
		// previous value
		obsolete = false
	}
	for _, li := range app.langInfos {
		if li.Code == langCode {
			li.addTranslation(str, trans)
		} else {
			li.addUntranslatedIfNotExists(str)
		}
	}
	if !exists {
		app.allStrings[str] = obsolete
	}
}

type AppState struct {
	Users []*User
	Apps  []*App
}

var (
	appState  = AppState{}
	staticDir = "static"

	tmplMain        = "main.html"
	tmplApp         = "app.html"
	tmplAppTrans    = "apptrans.html"
	tmplBase        = "base.html"
	templateNames   = [...]string{tmplMain, tmplApp, tmplAppTrans, tmplBase}
	templatePaths   = make([]string, 0)
	templates       *template.Template
	reloadTemplates = true
)

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

// we ignore errors when reading
func readTranslationsFromLog(app *App) error {
	path := app.translationLogFilePath()
	if !FileExists(path) {
		return errors.New(fmt.Sprintf("Translations log '%s' doesn't exist", path))
	}

	translations := ReadTranslation(path)
	for _, logentry := range translations {
		app.addTranslation(logentry.LangCode, logentry.EnglishStr, logentry.NewTranslation, true)
	}
	fmt.Printf("Found %d translation log entries for app %s\n", len(translations), app.config.Name)
	return nil
}

func serve404(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprint(w, `<html><body>Page Not Found!</body></html>`)
	http.NotFound(w, r)
}

func serverErrorMsg(w http.ResponseWriter, msg string) {
	fmt.Fprintf(w, `<html><body>Error: %s</body></html>`, msg)
}

func serveFileFromDir(w http.ResponseWriter, r *http.Request, dir, fileName string) {
	filePath := filepath.Join(dir, fileName)
	if !FileExists(filePath) {
		fmt.Printf("serveFileFromDir() file=%s doesn't exist\n", filePath)
	}
	http.ServeFile(w, r, filePath)
	/*
		b, err := ioutil.ReadFile(filePath)
		if err != nil {
			fmt.Printf("serveFileFromDir() file=%s doesn't exist\n", filePath)
			serve404(w, r)
			return
		}
		w.Write(b)
		fmt.Printf("serveFileFromDir() served %d bytes of '%s'\n", len(b), filePath)
	*/
}

func serveFileStatic(w http.ResponseWriter, r *http.Request, fileName string) {
	serveFileFromDir(w, r, staticDir, fileName)
}

const lenStatic = len("/s/")

// handler for url: /s/
func handleStatic(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[lenStatic:]
	serveFileStatic(w, r, file)
}

type ModelMain struct {
	Apps        *[]*App
	User        string
	UserIsAdmin bool
	ErrorMsg    string
	RedirectUrl string
}

type content struct {
	ContentHTML template.HTML
}

type templateParser struct {
	HTML string
}

func (tP *templateParser) Write(p []byte) (n int, err error) {
	tP.HTML += string(p)
	return len(p), nil
}

func isTopLevelUrl(url string) bool {
	return 0 == len(url) || "/" == url
}

// handler for url: /
func handleMain(w http.ResponseWriter, r *http.Request) {
	if !isTopLevelUrl(r.URL.Path) {
		serve404(w, r)
		return
	}
	user := decodeUserFromCookie(r)
	model := &ModelMain{Apps: &appState.Apps, User: user, UserIsAdmin: false, RedirectUrl: r.URL.String()}
	tp := &templateParser{}
	if err := GetTemplates().ExecuteTemplate(tp, tmplMain, model); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	content := &content{template.HTML(tp.HTML)}
	if err := GetTemplates().ExecuteTemplate(w, tmplBase, content); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func appAlreadyExists(appName string) bool {
	for _, app := range appState.Apps {
		if app.config.Name == appName {
			return true
		}
	}
	return false
}

func appInvalidField(app *App) string {
	app.config.Name = strings.TrimSpace(app.config.Name)
	if app.config.Name == "" {
		return "Name"
	}
	if app.config.DataDir == "" {
		return "DataDir"
	}
	if app.config.AdminTwitterUser == "" {
		return "AdminTwitterUser"
	}
	if app.config.UploadSecret == "" {
		return "UploadSecret"
	}
	return ""
}

func readAppData(app *App) error {
	if err := readTranslationsFromLog(app); err != nil {
		return err
	}
	return nil
}

func addApp(app *App) error {
	if invalidField := appInvalidField(app); invalidField != "" {
		return errors.New(fmt.Sprintf("App has invalid field '%s'", invalidField))
	}
	if appAlreadyExists(app.config.Name) {
		return errors.New("App already exists")
	}
	if err := readAppData(app); err != nil {
		return err
	}
	appState.Apps = append(appState.Apps, app)
	return nil
}

func (app *App) Name() string {
	return app.config.Name
}

func (app *App) Url() string {
	return app.config.Url
}

func findApp(name string) *App {
	for _, app := range appState.Apps {
		if app.config.Name == name {
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
	App         *App
	Langs       []*LangInfo
	User        string
	UserIsAdmin bool
	RedirectUrl string
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

func userIsAdmin(app *App, user string) bool {
	return user == app.config.AdminTwitterUser
}

func buildModelApp(app *App, user string) *ModelApp {
	model := &ModelApp{App: app, User: user, UserIsAdmin: userIsAdmin(app, user)}
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

// handler for url: /app?name=$app
func handleApp(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("name"))
	app := findApp(appName)
	if app == nil {
		serverErrorMsg(w, fmt.Sprintf("Application \"%s\" doesn't exist", appName))
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
		serverErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, fmt.Sprintf("AppTranslator: %s\n", app.config.Name))
	m := make(map[string][]LangTrans)
	for _, li := range app.langInfos {
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

// handler for url: /edittranslation
func handleEditTranslation(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	app := findApp(appName)
	if app == nil {
		serverErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	langCode := strings.TrimSpace(r.FormValue("lang"))
	if !IsValidLangCode(langCode) {
		serverErrorMsg(w, fmt.Sprintf("Invalid lang code '%s'", langCode))
		return
	}

	user := decodeUserFromCookie(r)
	if user == "" {
		serverErrorMsg(w, "User doesn't exist")
		return
	}
	str := strings.TrimSpace(r.FormValue("string"))
	translation := strings.TrimSpace(r.FormValue("translation"))
	//fmt.Printf("Adding translation: '%s'=>'%s', lang='%s'", str, translation, langCode)
	app.addTranslation(langCode, str, translation, false)
	app.writeTranslationToLog(langCode, str, translation, user)
	model := buildModelAppTranslations(app, langCode, decodeUserFromCookie(r))
	model.ShowTranslationEditedMsg = true
	if err := GetTemplates().ExecuteTemplate(w, tmplAppTrans, model); err != nil {
		fmt.Print(err.Error(), "\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
		serverErrorMsg(w, fmt.Sprintf("Application '%s' doesn't exist", appName))
		return
	}
	secret := strings.TrimSpace(r.FormValue("secret"))
	if secret != app.config.UploadSecret {
		serverErrorMsg(w, fmt.Sprintf("Invalid secret for app '%s'", appName))
		return
	}
	file, _, err := r.FormFile("strings")
	if err != nil {
		serverErrorMsg(w, fmt.Sprintf("No file with strings %s", err.Error()))
		return
	}
	defer file.Close()
	newStrings := parseUploadedStrings(file)
	if nil == newStrings {
		serverErrorMsg(w, "Error parsing uploaded strings")
		return
	}
	w.Write([]byte("Ok\n"))
}

type SecureCookieValue struct {
	User        string
	TwitterTemp string
}

func setSecureCookie(w http.ResponseWriter, cookieVal *SecureCookieValue) {
	val := make(map[string]string)
	val["user"] = cookieVal.User
	val["twittertemp"] = cookieVal.TwitterTemp
	if encoded, err := secureCookie.Encode(cookieName, val); err == nil {
		// TODO: set expiration (Expires    time.Time) long time in the future?
		cookie := &http.Cookie{
			Name:  cookieName,
			Value: encoded,
			Path:  "/",
		}
		http.SetCookie(w, cookie)
	} else {
		fmt.Printf("setSecureCookie(): error encoding secure cookie %s\n", err.Error())
	}
}

// to delete the cookie value (e.g. for logging out), we need to set an
// invalid value
func deleteSecureCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:   cookieName,
		Value:  "deleted",
		MaxAge: -1,
		Path:   "/",
	}
	http.SetCookie(w, cookie)
}

func getSecureCookie(r *http.Request) *SecureCookieValue {
	var ret *SecureCookieValue
	if cookie, err := r.Cookie(cookieName); err == nil {
		// detect a deleted cookie
		if "deleted" == cookie.Value {
			return nil
		}
		val := make(map[string]string)
		if err = secureCookie.Decode(cookieName, cookie.Value, &val); err != nil {
			fmt.Printf("Error decoding cookie %s\n", err.Error())
			return nil
		}
		//fmt.Printf("Got cookie %q\n", val)
		ret = new(SecureCookieValue)
		var ok bool
		if ret.User, ok = val["user"]; !ok {
			fmt.Printf("Error decoding cookie, no 'user' field\n")
			return nil
		}
		if ret.TwitterTemp, ok = val["twittertemp"]; !ok {
			fmt.Printf("Error decoding cookie, no 'twittertemp' field\n")
			return nil
		}
	}
	return ret
}

func decodeUserFromCookie(r *http.Request) string {
	cookie := getSecureCookie(r)
	if nil == cookie {
		return ""
	}
	return cookie.User
}

func decodeTwitterTempFromCookie(r *http.Request) string {
	cookie := getSecureCookie(r)
	if nil == cookie {
		return ""
	}
	return cookie.TwitterTemp
}

// handler for url: GET /login?redirect=$redirect
func handleLogin(w http.ResponseWriter, r *http.Request) {
	redirect := strings.TrimSpace(r.FormValue("redirect"))
	if redirect == "" {
		serverErrorMsg(w, fmt.Sprintf("Missing redirect value for /login"))
		return
	}
	q := url.Values{
		"redirect": {redirect},
	}.Encode()

	cb := "http://" + r.Host + "/oauthtwittercb" + "?" + q
	//fmt.Printf("handleLogin: cb=%s\n", cb)
	tempCred, err := oauthClient.RequestTemporaryCredentials(http.DefaultClient, cb, nil)
	if err != nil {
		http.Error(w, "Error getting temp cred, "+err.Error(), 500)
		return
	}
	cookie := &SecureCookieValue{TwitterTemp: tempCred.Secret}
	setSecureCookie(w, cookie)
	http.Redirect(w, r, oauthClient.AuthorizationURL(tempCred, nil), 302)
}

// getTwitter gets a resource from the Twitter API and decodes the json response to data.
func getTwitter(cred *oauth.Credentials, urlStr string, params url.Values, data interface{}) error {
	if params == nil {
		params = make(url.Values)
	}
	oauthClient.SignParam(cred, "GET", urlStr, params)
	resp, err := http.Get(urlStr + "?" + params.Encode())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	bodyData, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("Get %s returned status %d, %s", urlStr, resp.StatusCode, bodyData)
	}
	//fmt.Printf("getTwitter(): json: %s\n", string(bodyData))
	return json.Unmarshal(bodyData, data)
}

// handler for url: GET /oauthtwittercb?redirect=$redirect
func handleOauthTwitterCallback(w http.ResponseWriter, r *http.Request) {
	//fmt.Printf("handleOauthTwitterCallback()\n")
	redirect := strings.TrimSpace(r.FormValue("redirect"))
	if redirect == "" {
		serverErrorMsg(w, fmt.Sprintf("Missing redirect value for /login"))
		return
	}
	tempCred := oauth.Credentials{
		Token: r.FormValue("oauth_token"),
	}
	tempCred.Secret = decodeTwitterTempFromCookie(r)
	if "" == tempCred.Secret {
		http.Error(w, "Error getting temp token secret from cookie, ", 500)
		return
	}
	//fmt.Printf("  tempCred.Secret: %s\n", tempCred.Secret)
	tokenCred, _, err := oauthClient.RequestToken(http.DefaultClient, &tempCred, r.FormValue("oauth_verifier"))
	if err != nil {
		http.Error(w, "Error getting request token, "+err.Error(), 500)
		return
	}

	//fmt.Printf("  tokenCred.Token: %s\n", tokenCred.Token)

	var info map[string]interface{}
	if err := getTwitter(
		tokenCred,
		"https://api.twitter.com/1/account/verify_credentials.json",
		nil,
		&info); err != nil {
		http.Error(w, "Error getting timeline, "+err.Error(), 500)
		return
	}
	if user, ok := info["screen_name"].(string); ok {
		//fmt.Printf("  username: %s\n", user)
		cookie := getSecureCookie(r)
		cookie.User = user
		setSecureCookie(w, cookie)
	}
	http.Redirect(w, r, redirect, 302)
}

// handler for url: GET /logout?redirect=$redirect
func handleLogout(w http.ResponseWriter, r *http.Request) {
	redirect := strings.TrimSpace(r.FormValue("redirect"))
	if redirect == "" {
		serverErrorMsg(w, fmt.Sprintf("Missing redirect value for /logout"))
		return
	}
	deleteSecureCookie(w)
	http.Redirect(w, r, redirect, 302)
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

func main() {
	flag.Parse()

	if err := readSecrets(*configPath); err != nil {
		log.Fatalf("Failed reading config file %s. %s\n", *configPath, err.Error())
	}

	app := NewApp(config.App)
	if err := addApp(app); err != nil {
		log.Fatalf("Failed to add the app: %s, err: %s\n", app.config.Name, err.Error())
		return
	}

	// for testing, add a dummy app if no apps exist
	if len(appState.Apps) == 0 {
		fmt.Printf("Added dummy SumatraPDF app")
	}

	http.HandleFunc("/s/", handleStatic)
	http.HandleFunc("/app", handleApp)
	http.HandleFunc("/downloadtranslations", handleDownloadTranslations)
	http.HandleFunc("/edittranslation", handleEditTranslation)
	http.HandleFunc("/uploadstrings", handleUploadStrings)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/oauthtwittercb", handleOauthTwitterCallback)
	http.HandleFunc("/logout", handleLogout)
	http.HandleFunc("/", handleMain)

	fmt.Printf("Running on %s\n", *httpAddr)
	if err := http.ListenAndServe(*httpAddr, nil); err != nil {
		fmt.Printf("http.ListendAndServer() failed with %s\n", err.Error())
	}
	fmt.Printf("Exited\n")
}
