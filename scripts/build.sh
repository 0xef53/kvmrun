#!/bin/bash
set -eu

declare -rx PROJECT_REPO="github.com/0xef53/kvmrun"
declare -rx GOOS="linux"
declare -rx GOBIN="$(pwd)/bin"

declare -r USER_GROUP="$(stat --printf "%u:%g" .)"

install -d "$GOBIN"
trap "chown -R $USER_GROUP $GOBIN" 0

go version
go fmt "${PROJECT_REPO}/..."
go install -v -ldflags "-s -w" "${PROJECT_REPO}/cmd/..."

exit 0
