#!/bin/bash

go build -o apptranslator_app *.go  || exit 1
./apptranslator_app
#go run *.go -log apptranslator.log
