#!/usr/bin/env bash
set -e

mkdir -p dist
go mod tidy

echo "Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o dist/signal-echo-linux-amd64 .

echo "Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui" -o dist/signal-echo-windows-amd64.exe .

echo "Done! Binaries are inside dist/"
