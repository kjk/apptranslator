export GOPATH=$GOPATH:`pwd`/ext
go build -o apptranslator_app main.go util.go translog.go handler_*.go langs.go 
