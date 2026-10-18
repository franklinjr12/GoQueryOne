#!/usr/bin/env bash
# Local CI for GoQueryOne (no GitHub Actions). Runs: fmt, vet (linux + windows amd64/386),
# unit tests (CGO off, no system packages needed) and cross-builds both Windows archs.
#
# Usage: scripts/check.sh [fmt|vet|test|build|all]   (default: all)
# Env:   VERSION (default: git describe or "dev"), DIST (default: dist)
set -euo pipefail
cd "$(dirname "$0")/.."

PKG=github.com/franklinjr12/GoQueryOne/internal/buildinfo
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}
COMMIT=${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}
DATE=${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}
DIST=${DIST:-dist}
LDFLAGS="-s -w -H=windowsgui -X ${PKG}.Version=${VERSION} -X ${PKG}.Commit=${COMMIT} -X ${PKG}.Date=${DATE}"

step() { printf '\n==> %s\n' "$*"; }

do_fmt() {
  step "gofmt"
  local out
  out=$(gofmt -l $(git ls-files '*.go' 2>/dev/null || find . -name '*.go' -not -path './vendor/*'))
  if [ -n "$out" ]; then
    echo "gofmt needed on:"; echo "$out"; return 1
  fi
}

do_vet() {
  step "go vet (host, CGO off)"
  CGO_ENABLED=0 go vet ./...
  for arch in amd64 386; do
    step "go vet (windows/$arch)"
    GOOS=windows GOARCH=$arch CGO_ENABLED=0 go vet ./...
  done
}

do_test() {
  step "go test (CGO off)"
  CGO_ENABLED=0 go test -count=1 -cover ./...
  if [ "${RACE:-0}" = "1" ]; then
    step "go test -race"
    CGO_ENABLED=1 go test -count=1 -race ./...
  fi
}

do_build() {
  mkdir -p "$DIST"
  step "build windows/amd64 -> $DIST/GoQueryOne-x64.exe ($VERSION)"
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o "$DIST/GoQueryOne-x64.exe" ./cmd
  step "build windows/386 -> $DIST/GoQueryOne-x86.exe ($VERSION)"
  GOOS=windows GOARCH=386 CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o "$DIST/GoQueryOne-x86.exe" ./cmd
  ls -l "$DIST"/GoQueryOne-x64.exe "$DIST"/GoQueryOne-x86.exe
}

case "${1:-all}" in
  fmt) do_fmt ;;
  vet) do_vet ;;
  test) do_test ;;
  build) do_build ;;
  all) do_fmt; do_vet; do_test; do_build; step "check OK" ;;
  *) echo "usage: $0 [fmt|vet|test|build|all]"; exit 2 ;;
esac
