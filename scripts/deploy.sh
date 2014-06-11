#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

GOOS=linux GOARCH=amd64 godep go build -o apptranslator_app_linux
fab deploy
