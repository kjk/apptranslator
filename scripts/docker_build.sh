#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

GOOS=linux GOARCH=amd64 go build -o apptranslator_linux

docker build --tag apptranslator:latest .
