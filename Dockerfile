# syntax=docker/dockerfile:1.6
# OPTIONAL: containerised build. The normal build is `make build` (or
# scripts/check.sh build) on any machine with Go 1.24+: no C compiler, no
# Docker needed. This file just runs the same cross-compilation in a container.
#
#   docker buildx build --output type=local,dest=out .
#   -> out/GoQueryOne-x64.exe, out/GoQueryOne-x86.exe

FROM golang:1.24-bookworm AS builder
ARG VERSION=dev
ARG COMMIT=unknown
ENV CGO_ENABLED=0 GOOS=windows
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    set -e; \
    LDF="-s -w -H=windowsgui -X github.com/franklinjr12/GoQueryOne/internal/buildinfo.Version=${VERSION} -X github.com/franklinjr12/GoQueryOne/internal/buildinfo.Commit=${COMMIT}"; \
    GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags "$LDF" -o /out/GoQueryOne-x64.exe ./cmd; \
    GOARCH=386   go build -buildvcs=false -trimpath -ldflags "$LDF" -o /out/GoQueryOne-x86.exe ./cmd

FROM scratch AS artifact
COPY --from=builder /out/ /
