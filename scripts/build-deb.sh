#!/bin/bash
set -eu

if [[ ! -f "debian/control" ]]; then
	echo "Not found: ./debian/control" >&2
	exit 1
fi

declare -rx DEBFULLNAME="Sergey Zhuravlev"
declare -rx DEBEMAIL="sergey@divpro.ru"

declare -r RESULT_DIR="$(pwd)/packages"

trap "chown -R $(stat --printf '%u:%g' debian) $RESULT_DIR" 0

declare -r GIT_COMMIT_REV="$(git show -s --format=%h)"
declare -r GIT_REMOTE_URL="$(git ls-remote --get-url)"

declare -r VER="$(bin/vmm version | awk -F',' '{gsub("v", "", $1); print $1}')"
declare -r REVISION="git$(git rev-list HEAD --count).${GIT_COMMIT_REV}"

declare -r TMPDIR="$(mktemp -d)"

cp -t "$TMPDIR" -a "debian"
cp -t "$TMPDIR" -a "bin"
cp -t "$TMPDIR" -a "scripts"
cp -t "$TMPDIR" -a "contrib"
cp -t "$TMPDIR" "Makefile"

cd "$TMPDIR"

rm -vf "debian/changelog"

dch \
    --create \
    --package "kvmrun" \
    -D "unstable" \
    -v "1:${VER}+${REVISION}" \
    "built from ${GIT_REMOTE_URL} (commit: ${GIT_COMMIT_REV}"

cp "contrib/kvmrund.service" "debian/kvmrun.kvmrund.service"

dh_clean && dpkg-buildpackage -uc -us

find ../ -maxdepth 1 -type f -name "*.deb" -exec mv -t "$RESULT_DIR" {} ";"

exit 0

