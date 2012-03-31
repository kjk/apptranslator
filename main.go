package main

import (
	"fmt"
	"os"
)

const (
	// this is where we store, in an append-only fashion,
	// information about users and projects. All in
	// one place because I expect this information to be
	// very small
	DataFileName = "apptranslator.dat"
)

type User struct {
	Login string
}

type Project struct {
	Name      string
	AdminUser string // corresponds to User.Login field
}

func readDataAtStartup(fileName string) error {
	if file, err := os.Open(fileName); err != nil {
		if file, err = os.Create(fileName); err != nil {
			return err
		}
		file.Close()
		return nil
	} else {
		// TODO: read the data
		file.Close()
	}
	return nil
}

func main() {
	if err := readDataAtStartup(DataFileName); err != nil {
		fmt.Printf("Failed to open data file %s. Can't proceed.\n", DataFileName)
		return
	}
	fmt.Printf("Read the data from %s\n", DataFileName)
}
