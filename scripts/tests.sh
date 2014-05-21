#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

cd store
go test -test.v=true

