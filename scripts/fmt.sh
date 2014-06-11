#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

go fmt
cd tools/listbackup
go fmt
