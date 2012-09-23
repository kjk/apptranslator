#!/bin/bash
cp translog.go util.go langs.go tools/importsumatra
cd tools/importsumatra
go build -o importsum *.go
if [ "$?" -ne 0 ]; then echo "failed to build"; exit 1; fi 
rm translog.go util.go langs.go # we used you so now we discard you
cd ../..
./tools/importsumatra/importsum
rm tools/importsumatra/importsum
