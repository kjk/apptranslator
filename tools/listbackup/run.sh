#!/bin/bash
export GOPATH=`pwd`/ext:$GOPATH
cd tools/listbackup
go build -o listbackup *.go
if [ "$?" -ne 0 ]; then echo "failed to build"; exit 1; fi 
#rm translog.go util.go langs.go # we used you so now we discard you
cd ../..
./tools/listbackup/listbackup
rm tools/listbackup/listbackup
