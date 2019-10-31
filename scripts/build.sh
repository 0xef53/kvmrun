#!/bin/bash
set -eu

if [[ -z "${GOPATH:-}" ]] ; then
	echo "Undefined GOPATH variable" >&2
	exit 1
fi

declare -r PROJECT_REPO="github.com/0xef53/kvmrun"
declare -r GOOS="linux"

declare -r USER_GROUP="$(stat --printf "%u:%g" ${GOPATH}/src/${PROJECT_REPO})"
trap "chown -R $USER_GROUP ${GOPATH}/bin" 0

go version
gofmt -w "${GOPATH}/src/${PROJECT_REPO}"
go get -v -ldflags "-s -w" "${PROJECT_REPO}/..."

exit 0
