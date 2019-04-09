#!/bin/bash

set -e -x

go tool dist env > /tmp/goenv.tmp
. /tmp/goenv.tmp

VERSION=`grep " *v *[0-9]*\.[0-9]*\.[0-9]" CHANGELOG.md | sed 's/# v *\([0-9]*\.[0-9]*\.[0-9]\) .*/\1/g'|head -1`
COMMIT=`git rev-parse --short HEAD`


if [ ! -f dist/syncflux-${VERSION}-${COMMIT}_${GOOS:-linux}_${GOARCH:-amd64}.tar.gz ]
then
    echo "building binary...."
    go run build.go build-static
    go run build.go pkg-min-tar
else
    echo "skiping build..."
fi

export VERSION
export COMMIT

cp dist/syncflux-${VERSION}-${COMMIT}_${GOOS:-linux}_${GOARCH:-amd64}.tar.gz docker/syncflux-last.tar.gz
cp conf/sample.syncflux.toml docker/syncflux.toml

cd docker

sudo docker build --label version="${VERSION}" --label commitid="${COMMIT}" -t tonimoreno/syncflux:${VERSION} -t tonimoreno/syncflux:latest .
rm syncflux-last.tar.gz
rm syncflux.toml
rm /tmp/goenv.tmp
