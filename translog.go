package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type LogTranslationChange struct {
	LangCode       string `json:"l"`
	EnglishStr     string `json:"s"`
	NewTranslation string `json:"t"`
	User           string `json:"u"`
}

// must suceed or we exit
func ReadTranslation(path string) []LogTranslationChange {
	trans := make([]LogTranslationChange, 0, 2048)
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("readTranslationsFromLog(): file '%s' doesn't exist\n", path)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	var e LogTranslationChange
	for {
		err := decoder.Decode(&e)
		if err != nil {
			fmt.Printf("Finished decoding, %s\n", err.Error())
			break
		}
		if e.User == "" {
			e.User = "unknown"
		}
		trans = append(trans, e)
	}
	return trans
}
