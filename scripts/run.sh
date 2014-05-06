#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

go build -o apptranslator_app *.go
./apptranslator_app
#go run *.go -log apptranslator.log
