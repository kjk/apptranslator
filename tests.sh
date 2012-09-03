export GOPATH=$GOPATH:`pwd`/ext
go test langs.go translog.go translog2.go util.go maincommon.go main.go all_test.go
