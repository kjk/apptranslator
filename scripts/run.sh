export GOPATH=$GOPATH:`pwd`/ext
go run handler_*.go langs.go translog.go util.go main.go
#go run handler_*.go langs.go translog.go util.go main.go -log apptranslator.log
