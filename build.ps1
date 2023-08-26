$env:GOARCH="amd64"
$env:GOOS="windows"

go build -ldflags="-w -s" .

dir *.exe