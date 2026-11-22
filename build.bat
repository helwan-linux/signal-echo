@echo off
mkdir dist 2>nul
go mod tidy

echo Building for Windows (amd64)...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-H windowsgui" -o dist\signal-echo-windows-amd64.exe .

echo Building for Linux (amd64)...
set GOOS=linux
set GOARCH=amd64
go build -o dist\signal-echo-linux-amd64 .

echo Done! Binaries are inside dist\
pause
