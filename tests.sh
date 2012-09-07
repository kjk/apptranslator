export GOPATH=$GOPATH:`pwd`/ext
go test langs.go translog.go util.go all_test.go
