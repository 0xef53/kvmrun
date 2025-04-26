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

for NAME in "kvmrund" "vmm" "launcher" "gencert" "proxy-launcher" "printpci" "update-kvmrun-package" ; do
    go install \
        -v \
        -buildvcs=false \
        -ldflags "-s -w" \
        "${PROJECT_REPO}/cmd/${NAME}"
done

for NAME in "netinit" "vnetctl" ; do
    go install \
        -v \
        -buildvcs=false \
        -buildmode=pie \
        -tags 'netgo,osusergo' \
        -ldflags "-s -w -linkmode external -extldflags -static-pie" \
        "${PROJECT_REPO}/cmd/${NAME}"
done

exit 0
