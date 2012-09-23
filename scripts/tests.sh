export GOPATH=`pwd`/ext:$GOPATH
go test langs.go translog.go util.go all_test.go
