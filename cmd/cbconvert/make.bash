#!/usr/bin/env bash

GLIBC_x86_64="/usr/x86_64-pc-linux-gnu-static"
MINGW_x86_64="/usr/x86_64-w64-mingw32"
MACOS_x86_64="/usr/x86_64-apple-darwin"
MACOS_aarch64="/usr/aarch64-apple-darwin"

VERSION="$(git --git-dir ../../.git describe --tags --abbrev=0 2>/dev/null || echo '1.0.0')"

BUILDDIR="cbconvert-${VERSION}"

mkdir -p "${BUILDDIR}"
CC=x86_64-pc-linux-gnu-gcc \
PKG_CONFIG="x86_64-pc-linux-gnu-pkg-config" \
PKG_CONFIG_PATH="$GLIBC_x86_64/usr/lib64/pkgconfig" \
PKG_CONFIG_LIBDIR="$GLIBC_x86_64/usr/lib64/pkgconfig" \
CGO_CFLAGS="-I$GLIBC_x86_64/usr/include" \
CGO_LDFLAGS="-L$GLIBC_x86_64/usr/lib64" \
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
go build -trimpath -tags 'extlib pkgconfig' -v -o "${BUILDDIR}"/cbconvert -ldflags "-linkmode external -s -w -X main.appVersion=${VERSION} '-extldflags=-static'" && \
cp ../../README.md ../../AUTHORS ../../COPYING "${BUILDDIR}" && tar -czf "${BUILDDIR}-linux-x86_64.tar.gz" "${BUILDDIR}"
rm -rf "${BUILDDIR}"

mkdir -p "${BUILDDIR}"
CC=x86_64-w64-mingw32-gcc \
PKG_CONFIG="/usr/bin/x86_64-w64-mingw32-pkg-config" \
PKG_CONFIG_PATH="$MINGW_x86_64/usr/lib/pkgconfig" \
PKG_CONFIG_LIBDIR="$MINGW_x86_64/usr/lib/pkgconfig" \
CGO_CFLAGS="-I$MINGW_x86_64/usr/include" \
CGO_LDFLAGS="-L$MINGW_x86_64/usr/lib -ljxl -ljxl_dec -ljxl_profiler -ljxl_threads" \
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
go build -trimpath -tags 'extlib pkgconfig' -v -o "${BUILDDIR}"/cbconvert.exe -ldflags "-s -w -X main.appVersion=${VERSION} '-extldflags=-static -Wl,--allow-multiple-definition'" && \
cp ../../README.md ../../AUTHORS ../../COPYING "${BUILDDIR}" && zip -rq "${BUILDDIR}-windows-x86_64.zip" "${BUILDDIR}"
rm -rf "${BUILDDIR}"

export OSXCROSS_PKG_CONFIG_USE_NATIVE_VARIABLES=1
mkdir -p "${BUILDDIR}"
PATH=${PATH}:${MACOS_x86_64}/bin \
CC=x86_64-apple-darwin21.1-clang \
PKG_CONFIG="x86_64-apple-darwin21.1-pkg-config" \
PKG_CONFIG_PATH="$MACOS_x86_64/SDK/MacOSX12.1.sdk/usr/lib/pkgconfig" \
PKG_CONFIG_LIBDIR="$MACOS_x86_64/SDK/MacOSX12.1.sdk/usr/lib/pkgconfig" \
CGO_CFLAGS="-I$MACOS_x86_64/usr/include -I$MACOS_x86_64/macports/pkgs/opt/local/include" \
CGO_LDFLAGS="-L$MACOS_x86_64/SDK/MacOSX12.1.sdk/usr/lib -L$MACOS_x86_64/macports/pkgs/opt/local/lib -mmacosx-version-min=10.15" \
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
go build -trimpath -tags 'extlib pkgconfig' -v -o "${BUILDDIR}"/cbconvert -ldflags "-linkmode external -s -w -X main.appVersion=${VERSION}" && \
cp ../../README.md ../../AUTHORS ../../COPYING "${BUILDDIR}" && zip -rq "${BUILDDIR}-darwin-x86_64.zip" "${BUILDDIR}"
rm -rf "${BUILDDIR}"

export OSXCROSS_PKG_CONFIG_USE_NATIVE_VARIABLES=1
mkdir -p "${BUILDDIR}"
PATH=${PATH}:${MACOS_aarch64}/bin \
CC=aarch64-apple-darwin21.1-clang \
PKG_CONFIG="aarch64-apple-darwin21.1-pkg-config" \
PKG_CONFIG_PATH="$MACOS_aarch64/SDK/MacOSX12.1.sdk/usr/lib/pkgconfig" \
PKG_CONFIG_LIBDIR="$MACOS_aarch64/SDK/MacOSX12.1.sdk/usr/lib/pkgconfig" \
CGO_CFLAGS="-I$MACOS_aarch64/usr/include -I$MACOS_aarch64/macports/pkgs/opt/local/include" \
CGO_LDFLAGS="-L$MACOS_aarch64/SDK/MacOSX12.1.sdk/usr/lib -L$MACOS_aarch64/macports/pkgs/opt/local/lib -mmacosx-version-min=10.15" \
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
go build -trimpath -tags 'extlib pkgconfig' -v -o "${BUILDDIR}"/cbconvert -ldflags "-linkmode external -s -w -X main.appVersion=${VERSION}" && \
cp ../../README.md ../../AUTHORS ../../COPYING "${BUILDDIR}" && zip -rq "${BUILDDIR}-darwin-aarch64.zip" "${BUILDDIR}"
rm -rf "${BUILDDIR}"
