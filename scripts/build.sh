#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

go build -o apptranslator_app *.go
rm ./apptranslator_app
