export GOPATH=$GOPATH:`pwd`/ext
go build -o apptranslator main.go util.go translog.go translog2.go langs.go 
