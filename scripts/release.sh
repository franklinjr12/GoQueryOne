#!/usr/bin/env bash
# Build local release archives. Nothing is uploaded, tagged or published:
# Franklin reviews, tags and publishes by hand.
#
# Usage: scripts/release.sh VERSION        (e.g. 1.0.0; "v" prefix optional)
# Output (in $DIST, default dist/):
#   GoQueryOne-VERSION-windows-x64.zip, GoQueryOne-VERSION-windows-x86.zip, SHA256SUMS.txt
# Env: SKIP_CHECK=1 skips fmt/vet/tests (builds still run).
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION=${1:-${VERSION:-}}
VERSION=${VERSION#v}
if [ -z "$VERSION" ]; then
  echo "usage: $0 VERSION" >&2
  exit 2
fi
DIST=${DIST:-dist}
export VERSION DIST

if [ -n "$(git status --porcelain --untracked-files=no 2>/dev/null)" ]; then
  echo "warning: working tree has uncommitted changes; the build is not reproducible from a commit" >&2
fi

# Windows version resources want a.b.c.d; take the leading numeric part.
numeric=$(printf '%s' "$VERSION" | grep -oE '^[0-9]+(\.[0-9]+){0,3}' || true)
if [ -n "$numeric" ]; then
  IFS=. read -r a b c d <<<"$numeric"
  winver="${a:-0}.${b:-0}.${c:-0}.${d:-0}"
  echo "==> version resources $winver"
  restore() { git checkout -- cmd/rsrc_windows_amd64.syso cmd/rsrc_windows_386.syso 2>/dev/null || true; }
  trap restore EXIT
  (cd cmd && go run github.com/tc-hib/go-winres@v0.3.3 make --in ../winres/winres.json --arch amd64,386 \
      --file-version "$winver" --product-version "$winver")
fi

if [ "${SKIP_CHECK:-0}" = "1" ]; then
  scripts/check.sh build
else
  scripts/check.sh all
fi

stage=$(mktemp -d)
trap 'rm -rf "$stage"; ${restore_cmd:-true}' EXIT
[ -n "$numeric" ] && restore_cmd=restore

readme_txt() {
  cat <<TXT
GoQueryOne $VERSION ($1)
=====================================

Portable ODBC SQL client for Windows 10/11. No installation needed:
start GoQueryOne-$1.exe.

Use the x64 exe with 64-bit ODBC drivers/DSNs and the x86 exe with 32-bit
ones (error IM014 means the bitness does not match).

Settings are kept in config.json next to the exe when this folder is
writable (portable mode), otherwise in %APPDATA%\\GoQueryOne\\.

User guide:  USER_GUIDE.md (also Help > User guide, F1)
Changes:     CHANGELOG.md
License:     MIT, see LICENSE
Project:     https://github.com/franklinjr12/GoQueryOne

The exe is not code-signed; Windows SmartScreen may ask for confirmation
on the first start.
TXT
}

zips=()
for arch in x64 x86; do
  name="GoQueryOne-$VERSION-windows-$arch"
  dir="$stage/$name"
  mkdir -p "$dir"
  cp "$DIST/GoQueryOne-$arch.exe" "$dir/GoQueryOne-$arch.exe"
  cp LICENSE CHANGELOG.md "$dir/"
  cp docs/USER_GUIDE.md "$dir/USER_GUIDE.md"
  readme_txt "$arch" | sed 's/$/\r/' >"$dir/README.txt"
  rm -f "$DIST/$name.zip"
  python3 - "$stage" "$name" "$DIST/$name.zip" <<'PY'
import os, sys, zipfile
stage, name, out = sys.argv[1], sys.argv[2], sys.argv[3]
with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as z:
    root = os.path.join(stage, name)
    for f in sorted(os.listdir(root)):
        info = zipfile.ZipInfo(f"{name}/{f}", date_time=(2026, 1, 1, 0, 0, 0))
        info.compress_type = zipfile.ZIP_DEFLATED
        info.external_attr = 0o644 << 16
        with open(os.path.join(root, f), "rb") as fh:
            z.writestr(info, fh.read())
PY
  zips+=("$name.zip")
done

(cd "$DIST" && sha256sum "${zips[@]}" >SHA256SUMS.txt)
echo "==> release artifacts in $DIST (not published):"
(cd "$DIST" && ls -l "${zips[@]}" SHA256SUMS.txt && cat SHA256SUMS.txt)
