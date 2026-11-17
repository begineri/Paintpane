#!/usr/bin/env bash
# Regenerates every Paintpane icon / splash image and the exe icon resource.
#
#   bash assets-src/icon/export.sh            # from anywhere
#   PREVIEW=/tmp/icons.png bash assets-src/icon/export.sh   # also write a zoomed preview sheet
#
# Needs Go >= 1.22 (standard library only) and network access the first time,
# to fetch go-winres for the .syso. Put Go (and MinGW for the cgo build) on PATH first.
set -euo pipefail

WINRES_VERSION=v0.3.3   # github.com/tc-hib/go-winres, pinned

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"

# 1. Draw all images (assets-src/installer/Paintpane.ico, tray.png, app.png, about_splash.gif,
#    assets-src/installer/Paintpane.bmp, the intro site's icon-256.png and favicon-32.png).
(cd "$here" && go run . -root "$root" ${PREVIEW:+-preview "$PREVIEW"})

# 2. Embed assets-src/installer/Paintpane.ico into the Windows exe (icon only, no version info),
#    as the icon resource named GLFW_ICON (assets-src/installer/winres.json). GLFW gives its
#    window class that icon; without it the class falls back to Windows' generic application
#    icon, which the taskbar shows on the app's button. Explorer uses the same resource.
(cd "$root" && go run "github.com/tc-hib/go-winres@$WINRES_VERSION" make \
	--in assets-src/installer/winres.json --out cmd/spice/spice --arch amd64)

echo "done: icons written under $root"
