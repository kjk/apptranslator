package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"os"
)

func fmtArgs(args ...interface{}) string {
	if len(args) == 0 {
		return ""
	}
	format := args[0].(string)
	if len(args) == 1 {
		return format
	}
	return fmt.Sprintf(format, args[1:]...)
}

func panicWithMsg(defaultMsg string, args ...interface{}) {
	s := fmtArgs(args...)
	if s == "" {
		s = defaultMsg
	}
	fmt.Printf("%s\n", s)
	panic(s)
}

func fatalIfErr(err error, args ...interface{}) {
	if err == nil {
		return
	}
	panicWithMsg(err.Error(), args...)
}

func fatalIf(cond bool, args ...interface{}) {
	if !cond {
		return
	}
	panicWithMsg("fatalIf: condition failed", args...)
}

func http404(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func httpErrorf(w http.ResponseWriter, format string, args ...interface{}) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	http.Error(w, msg, http.StatusBadRequest)
}

func sha1OfFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		//fmt.Printf("os.Open(%s) failed with %s\n", path, err.Error())
		return nil, err
	}
	defer f.Close()
	h := sha1.New()
	_, err = io.Copy(h, f)
	if err != nil {
		//fmt.Printf("io.Copy() failed with %s\n", err.Error())
		return nil, err
	}
	return h.Sum(nil), nil
}

func sha1HexOfFile(path string) (string, error) {
	sha1, err := sha1OfFile(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha1), nil
}

func sha1HexOfBytes(data []byte) string {
	return fmt.Sprintf("%x", sha1OfBytes(data))
}

func sha1OfBytes(data []byte) []byte {
	h := sha1.New()
	h.Write(data)
	return h.Sum(nil)
}
