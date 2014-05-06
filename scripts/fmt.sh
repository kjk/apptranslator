#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

go fmt
cd tools/importsumatra
go fmt
cd ../listbackup
go fmt
