export GOPATH=$GOPATH:`pwd`/ext
go build -o apptranslator main.go util.go translog.go handler_*.go langs.go 
