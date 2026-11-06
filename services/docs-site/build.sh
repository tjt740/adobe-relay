#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
cp ../../docs/ADOBE_IMAGE_CLIENT_GUIDE.md guide.md
go test -race ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o "${1:?output binary path}" .
