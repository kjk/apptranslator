// This code is under BSD license. See license-bsd.txt
package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/kjk/apptranslator/store"
	atom "github.com/thomas11/atomgenerator"
)

const tmplRssAll = `
Recent {{.AppName}} translations:
<ul>
{{range .Translations}}
<li>'{{.User}}' translated '{{.Text}}' as '{{.Translation}}' in language {{.Lang}}</li>
{{end}}
</ul>
`

const tmplRssOneLang = `
<p>Untranslated strings: {{.UntranslatedCount}}</p>

<p>Recent {{.AppName}} translations for language {{.Lang}}
<ul>
{{range .Translations}}
<li>'{{.User}}' translated '{{.Text}}' as '{{.Translation}}' in language {{.Lang}}</li>
{{end}}
</ul>
</p>
`

var tRssAll = template.Must(template.New("rssall").Parse(tmplRssAll))
var tRssForLang = template.Must(template.New("rssforlang").Parse(tmplRssOneLang))

type RssModel struct {
	AppName      string
	Translations []store.Edit
	// only valid for tmplRssOneLang
	Lang              string
	UntranslatedCount int
}

// returns "" on error
func templateToString(t *template.Template, data interface{}) string {
	var buf bytes.Buffer
	err := t.Execute(&buf, data)
	if err != nil {
		logger.Errorf("Failed to execute template %q, error: %s", t.Name(), err)
		return ""
	}
	return string(buf.Bytes())
}

func getRssAll(app *App) string {
	edits := app.store.RecentEdits(10)
	pubTime := time.Now()
	if len(edits) > 0 {
		pubTime = edits[0].Time
	}

	title := fmt.Sprintf("%s translations on AppTranslator.org", app.Name)
	// TODO: technically should url-escape
	link := fmt.Sprintf("http://www.apptranslator.org/rss?app=%s", app.Name)
	feed := &atom.Feed{
		Title:   title,
		Link:    link,
		PubDate: pubTime,
	}
	model := &RssModel{AppName: app.Name, Translations: edits}
	html := templateToString(tRssAll, model)
	link = fmt.Sprintf("http://www.apptranslator.org/app/%s", app.Name)
	e := &atom.Entry{
		Title:       title,
		Link:        link,
		Description: html,
		PubDate:     pubTime}
	feed.AddEntry(e)

	s, err := feed.GenXml()
	if err != nil {
		return "Failed to generate XML feed"
	}
	return string(s)
}

func getRssForLang(app *App, lang string) string {
	pubTime := time.Now()
	edits := app.store.EditsForLang(lang, 10)
	if len(edits) > 0 {
		pubTime = edits[0].Time
	}

	title := fmt.Sprintf("%s %s translations on AppTranslator.org", app.Name, lang)
	// TODO: technically should url-escape
	link := fmt.Sprintf("http://www.apptranslator.org/rss?app=%s&lang=%s", app.Name, lang)
	feed := &atom.Feed{
		Title:   title,
		Link:    link,
		PubDate: pubTime,
	}

	model := &RssModel{AppName: app.Name, Translations: edits}
	model.UntranslatedCount = app.store.UntranslatedForLang(lang)
	html := templateToString(tRssForLang, model)
	title = fmt.Sprintf("%d missing %s %s translations", model.UntranslatedCount, app.Name, lang)
	link = fmt.Sprintf("http://www.apptranslator.org/app/%s/%s", app.Name, lang)
	e := &atom.Entry{
		Title:       title,
		Link:        link,
		Description: html,
		PubDate:     pubTime}
	feed.AddEntry(e)

	s, err := feed.GenXml()
	if err != nil {
		return "Failed to generate XML feed"
	}
	return string(s)
}

// url: /rss?app=$app[&lang=$lang]
func handleRss(w http.ResponseWriter, r *http.Request) {
	appName := strings.TrimSpace(r.FormValue("app"))
	app := findApp(appName)
	if app == nil {
		serveErrorMsg(w, fmt.Sprintf("Application \"%s\" doesn't exist", appName))
		return
	}
	lang := strings.TrimSpace(r.FormValue("lang"))
	if 0 == len(lang) {
		s := getRssAll(app)
		w.Write([]byte(s))
		return
	}

	if !store.IsValidLangCode(lang) {
		serveErrorMsg(w, fmt.Sprintf("Language \"%s\" is not valid", lang))
		return
	}

	w.Write([]byte(getRssForLang(app, lang)))
}
