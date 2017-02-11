#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

. ./scripts/docker_build.sh

docker run --rm -it -v ~/data/apptranslator:/data -p 5020:80 apptranslator:latest
