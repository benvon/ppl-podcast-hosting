#!/usr/bin/env bash

set -euo pipefail

go_root=$(mise where go)
export GOROOT="$go_root"
export PATH="$go_root/bin:$PATH"

go tool golangci-lint run ./...
go test ./...
node --test scripts/*.test.cjs
go tool gosec ./...
go tool govulncheck ./...
go run ./cmd/pplsite validate --config config/show.yaml --episodes episodes

build_dir=$(mktemp -d)
trap 'rm -rf "$build_dir"' EXIT
go run ./cmd/pplsite build --config config/show.yaml --episodes episodes --out "$build_dir"
