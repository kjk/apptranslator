#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

go fmt *.go
go fmt tools/listbackup/*.go

