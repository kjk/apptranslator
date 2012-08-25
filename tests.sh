export GOPATH=$GOPATH:`pwd`/ext
go test langs.go translog.go translog2.go util.go main.go all_test.go
