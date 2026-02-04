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

go install \
    -v \
    -buildvcs=false \
    -ldflags "-s -w" \
        "${PROJECT_REPO}/cmd/kvmrund" \
        "${PROJECT_REPO}/cmd/vmm" \
        "${PROJECT_REPO}/cmd/launcher" \
        "${PROJECT_REPO}/cmd/gencert" \
        "${PROJECT_REPO}/cmd/printpci" \
        "${PROJECT_REPO}/cmd/update-kvmrun-package"

go install \
    -v \
    -buildvcs=false \
    -buildmode=pie \
    -tags 'netgo,osusergo' \
    -ldflags "-s -w -linkmode external -extldflags -static-pie" \
        "${PROJECT_REPO}/cmd/netinit" \
        "${PROJECT_REPO}/cmd/vnetctl"

exit 0
