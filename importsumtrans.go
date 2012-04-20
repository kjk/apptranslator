package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

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
		fmt.Printf("&Lang{\"%s\", \"%s\", \"%s\"},\n", lt.LangId, lt.LangNameEnglish, lt.LangNameNative)
	}*/
}
