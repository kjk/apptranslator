export GOPATH=$GOPATH:`pwd`/ext
go test langs.go translog.go main.go all_test.go
