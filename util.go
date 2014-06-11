package main

import (
	"fmt"
	"net/http"
)

func panicif(shouldPanic bool, format string, args ...interface{}) {
	if shouldPanic {
		s := format
		if len(args) > 0 {
			s = fmt.Sprintf(format, args...)
		}
		panic(s)
	}
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
