package main

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"html/template"
	"strings"
	"errors"
	"fmt"
	"time"
	"flag"
	"io/ioutil"
	"oauth"
	"log"
	"path/filepath"
	"gorilla/securecookie"
)

const (
	cookieName = "ckie"
)

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

type AppState struct {
	Users []*User
	Apps  []*App
}

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

	httpAddr   = flag.String("addr", ":8089", "HTTP server address")

	staticDir = "static"

	appState  = AppState{}

	tmplMain        = "main.html"
	tmplApp         = "app.html"
	tmplAppTrans    = "apptrans.html"
	tmplBase        = "base.html"
	templateNames   = [...]string{tmplMain, tmplApp, tmplAppTrans, tmplBase}
	templatePaths   = make([]string, 0)
	templates       *template.Template
	reloadTemplates = true

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

func (a *App) translationLogFilePath() string {
	return filepath.Join(filepath.Join(getDataDir(), a.config.DataDir), "translations.dat")
}

func findApp(name string) *App {
	for _, app := range appState.Apps {
		if app.config.Name == name {
			return app
		}
	}
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
	//fmt.Fprint(w, `<html><body>Page Not Found!</body></html>`)
	http.NotFound(w, r)
}

func serveErrorMsg(w http.ResponseWriter, msg string) {
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
		serveErrorMsg(w, fmt.Sprintf("Missing redirect value for /login"))
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
		serveErrorMsg(w, fmt.Sprintf("Missing redirect value for /login"))
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
		serveErrorMsg(w, fmt.Sprintf("Missing redirect value for /logout"))
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

func makeTimingHandler(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Path
		startTime := time.Now()
		fn(w, r)
		duration := time.Now().Sub(startTime)
		if duration.Seconds() > 1.0 {
			// TODO: log this information somewhere else, like a file
			fmt.Printf("'%s' took %f seconds to serve\n", url, duration.Seconds())
		}
	}
}
