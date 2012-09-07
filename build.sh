export GOPATH=$GOPATH:`pwd`/ext
go build -o apptranslator main.go util.go translog.go handler_*.go langs.go 
go build -o importsumtrans translog.go util.go langs.go importsumtrans.go
rm importsumtrans apptranslator