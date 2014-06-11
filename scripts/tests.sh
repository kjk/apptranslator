#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

cd store
godep go test -test.v=true
