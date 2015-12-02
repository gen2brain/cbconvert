#!/usr/bin/env bash

CHROOT="/home/milann/chroot"
MINGW="/usr/i686-w64-mingw32"

mkdir -p build
rm -f resource.syso

LIBRARY_PATH="$CHROOT/usr/lib:$CHROOT/lib" \
PKG_CONFIG_PATH="$CHROOT/usr/lib/pkgconfig" \
PKG_CONFIG_LIBDIR="$CHROOT/usr/lib/pkgconfig" \
CGO_LDFLAGS="-L$CHROOT/usr/lib -L$CHROOT/lib" \
CC_FOR_TARGET="x86_64-pc-linux-gnu-gcc" CXX_FOR_TARGET="x86_64-pc-linux-gnu-g++" \
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -v -x -o build/cbconvert
strip build/cbconvert

go generate
PKG_CONFIG="/usr/bin/i686-w64-mingw32-pkg-config" \
PKG_CONFIG_PATH="$MINGW/usr/lib/pkgconfig" \
PKG_CONFIG_LIBDIR="$MINGW/usr/lib/pkgconfig" \
CGO_LDFLAGS="-L$MINGW/usr/lib" \
CGO_CFLAGS="-I$MINGW/usr/include -Wno-poison-system-directories" \
CGO_CXXFLAGS="-I$MINGW/usr/include -Wno-poison-system-directories" \
CGO_CPPFLAGS="-I$MINGW/usr/include -Wno-poison-system-directories" \
CC="i686-w64-mingw32-gcc" CXX="i686-w64-mingw32-g++" \
CC_FOR_TARGET="i686-w64-mingw32-gcc" CXX_FOR_TARGET="i686-w64-mingw32-g++" \
CGO_ENABLED=1 GOOS=windows GOARCH=386 go build -v -x -o build/cbconvert.exe -ldflags "-linkmode external '-extldflags=-static -Wl,--allow-multiple-definition'"
i686-w64-mingw32-strip build/cbconvert.exe
