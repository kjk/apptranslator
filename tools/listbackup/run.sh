#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

if [ -e tools/listbackup ]; then cd tools/listbackup; fi
go build -o listbackup *.go
cd ../..
./tools/listbackup/listbackup
rm tools/listbackup/listbackup
