#!/usr/bin/env bash

CHROOT="/home/milann/chroot"
MINGW="/usr/i686-w64-mingw32"

mkdir -p build
rm -f resource.syso

go generate

LIBRARY_PATH="$CHROOT/usr/lib:$CHROOT/lib" \
PKG_CONFIG_PATH="$CHROOT/usr/lib/pkgconfig" \
PKG_CONFIG_LIBDIR="$CHROOT/usr/lib/pkgconfig" \
CGO_LDFLAGS="-L$CHROOT/usr/lib -L$CHROOT/lib -L$CHROOT/usr/plugins/generic -L$CHROOT/usr/plugins/platforms -L$CHROOT/usr/plugins/qmltooling -L$CHROOT/usr/qml/QtQuick.2 -L$CHROOT/usr/qml/QtQuick/Controls -L$CHROOT/usr/qml/QtQuick/Dialogs -L$CHROOT/usr/qml/QtQuick/Layouts -L$CHROOT/usr/qml/QtQuick/Window.2 -L$CHROOT/usr/qml/Qt/labs/settings -L$CHROOT/usr/qml/QtQuick/PrivateWidgets -L$CHROOT/usr/plugins/xcbglintegrations" \
CGO_LDFLAGS="$CGO_LDFLAGS -lqxcb -lqtquick2plugin -lqtquickcontrolsplugin -ldialogplugin -lqquicklayoutsplugin -lwindowplugin -lqmlsettingsplugin -lwidgetsplugin -lqxcb-glx-integration -lqevdevkeyboardplugin -lqevdevmouseplugin" \
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -v -x -o build/cbconvert
strip build/cbconvert

goversioninfo -icon=assets/icon.ico

PKG_CONFIG="/usr/bin/i686-w64-mingw32-pkg-config" \
PKG_CONFIG_PATH="$MINGW/usr/lib/pkgconfig:$MINGW/usr/lib/pkgconfig" \
PKG_CONFIG_LIBDIR="$MINGW/usr/lib/pkgconfig:$MINGW/usr/lib/pkgconfig" \
CGO_LDFLAGS="-L$MINGW/usr/lib -L$MINGW/lib -L$MINGW/usr/plugins/generic -L$MINGW/usr/plugins/platforms -L$MINGW/usr/qml/Qt/labs/folderlistmodel -L$MINGW/usr/plugins/qmltooling -L$MINGW/usr/qml/QtQuick.2 -L$MINGW/usr/qml/QtQuick/Controls -L$MINGW/usr/qml/QtQuick/Dialogs -L$MINGW/usr/qml/QtQuick/Dialogs/Private -L$MINGW/usr/qml/QtQuick/Layouts -L$MINGW/usr/qml/QtQuick/Window.2 -L$MINGW/usr/qml/Qt/labs/settings -L$MINGW/usr/qml/QtQuick/PrivateWidgets" \
CGO_LDFLAGS="$CGO_LDFLAGS -lqwindows -lqtquick2plugin -lqtquickcontrolsplugin -lqmlfolderlistmodelplugin -ldialogplugin -ldialogsprivateplugin -lqquicklayoutsplugin -lwindowplugin -lqmlsettingsplugin -lwidgetsplugin" \
CGO_CFLAGS="-I$MINGW/usr/include -Wno-poison-system-directories" \
CGO_CXXFLAGS="-I$MINGW/usr/include -Wno-poison-system-directories" \
CGO_CPPFLAGS="-I$MINGW/usr/include -Wno-poison-system-directories" \
CC="i686-w64-mingw32-gcc" CXX="i686-w64-mingw32-g++" \
CC_FOR_TARGET="i686-w64-mingw32-gcc" CXX_FOR_TARGET="i686-w64-mingw32-g++" \
CGO_ENABLED=1 GOOS=windows GOARCH=386 go build -v -x -o build/cbconvert.exe -ldflags "-H=windowsgui -linkmode external '-extldflags=-static -Wl,--allow-multiple-definition'"
i686-w64-mingw32-strip build/cbconvert.exe
