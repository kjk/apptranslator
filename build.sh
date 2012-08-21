export GOPATH=$GOPATH:`pwd`/ext
go build -o apptranslator main.go util.go translog.go translog2.go langs.go 
go build -o importsumtrans translog2.go util.go langs.go importsumtrans.go
rm importsumtrans apptranslator