#!/usr/bin/env bash

MUSL="/usr/x86_64-pc-linux-musl"
MINGW="/usr/i686-w64-mingw32"

VERSION="`git --git-dir ../../.git describe --tags --abbrev=0 >/dev/null 2>&1 || echo '0.0.0'`"

BUILDDIR="cbconvert-${VERSION}"
mkdir -p ${BUILDDIR}

CC=x86_64-pc-linux-musl-gcc \
PKG_CONFIG="x86_64-pc-linux-musl-pkg-config" \
PKG_CONFIG_PATH="$MUSL/usr/lib/pkgconfig" \
PKG_CONFIG_LIBDIR="$MUSL/usr/lib/pkgconfig" \
CGO_CFLAGS="-I$MUSL/usr/include" \
CGO_LDFLAGS="-L$MUSL/usr/lib" \
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
go build -tags 'extlib static' -v -o ${BUILDDIR}/cbconvert -ldflags "-linkmode external -s -w '-extldflags=-static'"

cp ../../README.md ../../AUTHORS ../../COPYING ${BUILDDIR} && tar -czf "${BUILDDIR}-linux-x86_64.tar.gz" ${BUILDDIR} 
rm -rf ${BUILDDIR}


BUILDDIR="cbconvert-${VERSION}"
mkdir -p ${BUILDDIR}

CC=i686-w64-mingw32-gcc \
PKG_CONFIG="/usr/bin/i686-w64-mingw32-pkg-config" \
PKG_CONFIG_PATH="$MINGW/usr/lib/pkgconfig" \
PKG_CONFIG_LIBDIR="$MINGW/usr/lib/pkgconfig" \
CGO_CFLAGS="-I$MINGW/usr/include" \
CGO_LDFLAGS="-L$MINGW/usr/lib" \
CGO_ENABLED=1 GOOS=windows GOARCH=386 \
go build -tags 'extlib static' -v -o ${BUILDDIR}/cbconvert.exe -ldflags "-s -w '-extldflags=-static -Wl,--allow-multiple-definition'"

cp ../../README.md ../../AUTHORS ../../COPYING ${BUILDDIR} && zip -rq "${BUILDDIR}-windows-i686.zip" ${BUILDDIR}
rm -rf ${BUILDDIR}
