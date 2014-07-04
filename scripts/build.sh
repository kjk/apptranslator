#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

rm -rf $TMPDIR/godep
godep go build -o apptranslator_app *.go
rm ./apptranslator_app
