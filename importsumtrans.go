package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type LogTranslationChange struct {
	LangCode       string `json:"l"`
	EnglishStr     string `json:"s"`
	NewTranslation string `json:"t"`
	User           string `json:"u"`
}

func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

type TranslationLog struct {
	fileName string
	file     *os.File
	encoder  *json.Encoder
}

func NewTranslationLog(fileName string) *TranslationLog {
	if FileExists(fileName) {
		fmt.Printf("File '%s' already exists. We won't over-write it for safety.\n", fileName)
		return nil
	}
	file, err := os.Create(fileName)
	if err != nil {
		fmt.Printf("Couldn't create file '%s', %s\n", fileName, err.Error())
		return nil
	}
	encoder := json.NewEncoder(file)
	return &TranslationLog{fileName, file, encoder}
}

func (tl *TranslationLog) close() {
	tl.file.Close()
}

func (tl *TranslationLog) write(langCode, s, trans string) error {
	var logentry LogTranslationChange
	logentry.LangCode = langCode
	logentry.EnglishStr = s
	logentry.NewTranslation = trans
	err := tl.encoder.Encode(logentry)
	if err != nil {
		fmt.Printf("Error encode %s in %s\n", err.Error(), tl.fileName)
		return err
	}
	return nil
}

func main() {
	dir := "../sumatrapdf/strings"
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Printf("Error reading dir '%s', %s\n", dir, err.Error())
		return
	}
	log := NewTranslationLog("SumatraPDF_trans.dat")
	if log == nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		ParseFile(path, log)
	}
	log.close()
}
