#!/usr/bin/env bash
# Regenerates every Paintpane icon / splash image and the exe icon resource.
#
#   bash assets-src/icon/export.sh            # from anywhere
#   PREVIEW=/tmp/icons.png bash assets-src/icon/export.sh   # also write a zoomed preview sheet
#
# Needs Go >= 1.22 (standard library only) and network access the first time,
# to fetch rsrc for the .syso. Put Go (and MinGW for the cgo build) on PATH first.
set -euo pipefail

RSRC_VERSION=v0.10.2   # github.com/akavel/rsrc, pinned

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"

# 1. Draw all images (assets-src/installer/Paintpane.ico, tray.png, splash.png, about_splash.gif,
#    assets-src/installer/Paintpane.bmp).
(cd "$here" && go run . -root "$root" ${PREVIEW:+-preview "$PREVIEW"})

# 2. Embed assets-src/installer/Paintpane.ico into the Windows exe (icon only, no version info).
(cd "$root" && go run "github.com/akavel/rsrc@$RSRC_VERSION" \
	-ico assets-src/installer/Paintpane.ico -arch amd64 -o cmd/spice/spice_windows_amd64.syso)

echo "done: icons written under $root"
