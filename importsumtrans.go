package main

import (
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type LogTranslationChange struct {
	LangCode       string
	EnglishStr     string
	NewTranslation string
}

func FileExists(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

func writeTranslationLog(fileName string, trans []*LangTranslations) {
	if FileExists(fileName) {
		fmt.Printf("File '%s' already exists. We won't over-write it for safety.", fileName)
	}
	file, err := os.Create(fileName)
	if err != nil {
		fmt.Printf("Couldn't create file '%s', %s", fileName, err.Error())
		return
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	var logentry LogTranslationChange
	for _, t := range trans {
		logentry.LangCode = t.LangCode
		for i, s := range t.Strings {
			logentry.EnglishStr = s
			logentry.NewTranslation = t.Translations[i]
			err := encoder.Encode(logentry)
			if err != nil {
				fmt.Printf("Error encode %s", err.Error())
			}
		}
	}
}

func main() {
	dir := "../sumatrapdf/strings"
	trans := make([]*LangTranslations, 0)
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Printf("Error reading dir '%s', %s\n", dir, err.Error())
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			if strings.HasSuffix(e.Name(), ".txt") {
				path := filepath.Join(dir, e.Name())
				t, err := ParseFile(path)
				if err != nil {
					fmt.Printf("Error parsing file '%s', %s\n", path, err.Error())
				}
				trans = append(trans, t)
			}
		}
	}
	//fmt.Printf("Parsed %d files\n", len(trans))
	/*for _, lt := range trans {
		fmt.Printf("&Lang{\"%s\", \"%s\", \"%s\"},\n", lt.LangCode, lt.LangNameEnglish, lt.LangNameNative)
	}*/
	writeTranslationLog("SumatraPDF_trans.dat", trans)
}
