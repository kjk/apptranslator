#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

go test langs.go translog.go util.go all_test.go
