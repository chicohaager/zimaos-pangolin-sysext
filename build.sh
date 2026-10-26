#!/bin/sh
# Build the pangolin sysext for ZimaOS.
#   sh build.sh [amd64|arm64]      (default: amd64)
# Output in dist/: pangolin-<arch>.raw (+ .sha256), for amd64 also pangolin.raw.
set -eu
cd "$(dirname "$0")"
ARCH="${1:-amd64}"
MODKIT="${LINTUX_MODKIT:-../lintux-modkit}"
[ -f "$MODKIT/sysext/add-web.sh" ] || { echo "lintux-modkit with sysext/add-web.sh not found at $MODKIT (set LINTUX_MODKIT)" >&2; exit 1; }
VERSION=$(cat VERSION)

# Upstream release of the Pangolin CLI, pinned by version AND sha256 (the digests GitHub lists
# for the release assets, 2026-10-26), plus its source for the license check and the AGPL-3
# source offer (GitHub's tag archive, sha256 measured twice the same, 2026-10-26).
CLI_VERSION=0.19.0
case "$ARCH" in
  amd64) SUM=3bf2e111c1407fb7abd1493618a7aa5c3edec3519caeecf53b97dca016076d82 ;;
  arm64) SUM=2d73daf78fea3ca7419e16263bcc4ded3a3758addd320b14cd34ff35281e2690 ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 2 ;;
esac
SRC_SUM=4cc6fb432e6dd5ea48263988b60405c270e374112dc18ce5bdc7342437faf02d
BIN="pangolin-cli_linux_$ARCH"
SRC="cli-$CLI_VERSION.tar.gz"

rm -rf build
mkdir -p build/root build/dl
curl -fsSL -o "build/dl/$BIN" "https://github.com/fosrl/cli/releases/download/$CLI_VERSION/$BIN"
curl -fsSL -o "build/dl/$SRC" "https://github.com/fosrl/cli/archive/refs/tags/$CLI_VERSION.tar.gz"
printf '%s  %s\n%s  %s\n' "$SUM" "build/dl/$BIN" "$SRC_SUM" "build/dl/$SRC" | sha256sum -c -
tar -xzf "build/dl/$SRC" -C build/dl
sh tools/check-license.sh "build/dl/cli-$CLI_VERSION"
sh tools/wrapper-test.sh
go vet ./...
go test -count=1 ./...

cp -a files/. build/root/
chmod 755 build/root/usr/bin/pangolin build/root/usr/bin/pangolin-zima
mkdir -p build/root/usr/libexec/pangolin build/root/usr/share/doc/pangolin
install -m 755 "build/dl/$BIN" build/root/usr/libexec/pangolin/pangolin
cp "$MODKIT/sysext/unit-watchdog.sh" build/root/usr/libexec/pangolin/unit-watchdog.sh
cp "build/dl/cli-$CLI_VERSION/LICENSE" build/root/usr/share/doc/pangolin/LICENSE.pangolin-cli
printf 'Pangolin CLI %s (%s sha256 %s)\nsource: https://github.com/fosrl/cli/tree/%s (AGPL-3, %s sha256 %s)\n' \
  "$CLI_VERSION" "$BIN" "$SUM" "$CLI_VERSION" "$SRC" "$SRC_SUM" > build/root/usr/share/doc/pangolin/packages.txt
cp README.md build/root/usr/share/doc/pangolin/README
# The tile: module JSON (its "services" make zpkg enable and start pangolin-web; without the
# JSON zpkg answers "module not installed", rc=1 — measured 1.8.0), UI files and the backend.
sh "$MODKIT/sysext/add-web.sh" pangolin build/root "$VERSION"
# -buildvcs=false: without it Go stamps the nearest tag and "dirty" (any untracked file) into the binary,
# so a rebuild from the release tag differed from the release (measured 2026-10-23, restic-web).
CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -trimpath -buildvcs=false -ldflags="-s -w -X main.version=$VERSION" \
  -o build/root/usr/bin/pangolin-web ./cmd/pangolin-web
sh "$MODKIT/sysext/build-sysext.sh" pangolin build/root "$ARCH" dist "$VERSION"
cat "dist/pangolin-$ARCH.raw.sha256"
