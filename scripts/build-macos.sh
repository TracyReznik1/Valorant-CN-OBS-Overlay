#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
version="$(tr -d '[:space:]' < VERSION)"
go test ./...
mkdir -p build
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w -X main.version=$version" -o build/ValorantCNOverlay-macos-arm64 ./src/cmd/valorant-overlay
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$version" -o build/ValorantCNOverlay-macos-amd64 ./src/cmd/valorant-overlay
echo "Built: build/ValorantCNOverlay-macos-arm64 (v$version)"
echo "Built: build/ValorantCNOverlay-macos-amd64 (v$version)"
