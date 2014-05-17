// This code is under BSD license. See license-bsd.txt
package store

import "fmt"

type Lang struct {
	Code       string
	Name       string
	NameNative string
}

// list of languages that we know about
var (
	Languages = [...]*Lang{
		&Lang{"af", "Afrikaans", "Afrikaans"},
		&Lang{"am", "Armenian", "Հայերեն"},
		&Lang{"ar", "Arabic", "الْعَرَبيّة"},
		&Lang{"az", "Azerbaijani", "اآذربایجان دیلی"},
		&Lang{"bg", "Bulgarian", "Български"},
		&Lang{"bn", "Bengali", "বাংলা"},
		&Lang{"br", "Portuguese - Brazil", "Português"},
		&Lang{"bs", "Bosnian", "Bosanski"},
		&Lang{"by", "Belarusian", "Беларуская"},
		&Lang{"ca-xv", "Catalan-Valencian", "Català-Valencià"},
		&Lang{"ca", "Catalan", "Català"},
		&Lang{"cn", "Chinese Simplified", "简体中文"},
		&Lang{"cy", "Welsh", "Cymraeg"},
		&Lang{"cz", "Czech", "Čeština"},
		&Lang{"de", "German", "Deutsch"},
		&Lang{"dk", "Danish", "Dansk"},
		&Lang{"es", "Spanish", "Español"},
		&Lang{"et", "Estonian", "Eesti"},
		&Lang{"eu", "Basque", "Euskara"},
		&Lang{"fa", "Persian", "فارسی"},
		&Lang{"fi", "Finnish", "Suomi"},
		&Lang{"fr", "French", "Français"},
		&Lang{"fy-nl", "Frisian", "Frysk"},
		&Lang{"ga", "Irish", "Gaeilge"},
		&Lang{"gl", "Galician", "Galego"},
		&Lang{"el", "Greek", "Ελληνικά"},
		&Lang{"he", "Hebrew", "עברית"},
		&Lang{"hi", "Hindi", "हिंदी"},
		&Lang{"hr", "Croatian", "Hrvatski"},
		&Lang{"hu", "Hungarian", "Magyar"},
		&Lang{"id", "Indonesian", "Bahasa Indonesia"},
		&Lang{"it", "Italian", "Italiano"},
		&Lang{"ja", "Japanese", "日本語"},
		&Lang{"ka", "Georgian", "ქართული"},
		&Lang{"ku", "Kurdish", "كوردی"},
		&Lang{"kr", "Korean", "한국어"},
		&Lang{"kw", "Cornish", "Kernewek"},
		&Lang{"lt", "Lithuanian", "Lietuvių"},
		&Lang{"mk", "Macedonian", "македонски"},
		&Lang{"ml", "Malayalam", "മലയാളം"},
		&Lang{"mm", "Burmese", "ဗမာ စာ"},
		&Lang{"my", "Malaysian", "Bahasa Melayu"},
		&Lang{"ne", "Nepali", "नेपाली"},
		&Lang{"nl", "Dutch", "Nederlands"},
		&Lang{"nn", "Norwegian Neo-Norwegian", "Norsk nynorsk"},
		&Lang{"no", "Norwegian", "Norsk"},
		&Lang{"pa", "Punjabi", "ਪੰਜਾਬੀ"},
		&Lang{"pl", "Polish", "Polski"},
		&Lang{"pt", "Portuguese - Portugal", "Português"},
		&Lang{"ro", "Romanian", "Română"},
		&Lang{"ru", "Russian", "Русский"},
		&Lang{"si", "Sinhala", "සිංහල"},
		&Lang{"sk", "Slovak", "Slovenčina"},
		&Lang{"sl", "Slovenian", "Slovenščina"},
		&Lang{"sn", "Shona", "Shona"},
		&Lang{"sp-rs", "Serbian", "Latin"},
		&Lang{"sq", "Albanian", "Shqip"},
		&Lang{"sr-rs", "Serbian", "Cyrillic"},
		&Lang{"sv", "Swedish", "Svenska"},
		&Lang{"ta", "Tamil", "தமிழ்"},
		&Lang{"th", "Thai", "ภาษาไทย"},
		&Lang{"tl", "Tagalog", "Tagalog"},
		&Lang{"tr", "Turkish", "Türkçe"},
		&Lang{"tw", "Chinese Traditional", "繁體中文"},
		&Lang{"uk", "Ukrainian", "Українська"},
		&Lang{"uz", "Uzbek", "O'zbek"},
		&Lang{"vn", "Vietnamese", "Việt Nam"},
	}
)

func LangNameByCode(code string) string {
	for _, lang := range Languages {
		if code == lang.Code {
			return lang.Name
		}
	}
	return fmt.Sprintf("Unknown lang code %s", code)
}

func IsValidLangCode(code string) bool {
	for _, lang := range Languages {
		if code == lang.Code {
			return true
		}
	}
	return false
}
