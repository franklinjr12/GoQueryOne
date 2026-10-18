# GoQueryOne local CI / build. No GitHub Actions: run `make check` before pushing.
SHELL := /usr/bin/env bash
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
export VERSION

.PHONY: all check fmt vet test test-race test-integration build winres release clean

all: check

check: ## fmt + vet + test + build (the local "CI")
	scripts/check.sh all

fmt: ## fail if gofmt would change anything
	scripts/check.sh fmt

vet: ## go vet for host and windows/amd64 + windows/386
	scripts/check.sh vet

test: ## unit tests, CGO off, no system packages required
	scripts/check.sh test

test-race: ## unit tests with the race detector (needs cgo + a C compiler)
	RACE=1 scripts/check.sh test

test-integration: ## ODBC integration tests (needs unixODBC + a driver, cgo)
	CGO_ENABLED=1 go test -count=1 -tags integration ./...

build: ## dist/GoQueryOne-x64.exe and dist/GoQueryOne-x86.exe
	scripts/check.sh build

winres: ## regenerate cmd/rsrc_windows_*.syso from winres/winres.json (icon, manifest, version info)
	cd cmd && go run github.com/tc-hib/go-winres@v0.3.3 make --in ../winres/winres.json --arch amd64,386

release: ## local release zips + SHA256SUMS in dist/ (never published automatically)
	scripts/release.sh $(VERSION)

clean:
	rm -rf dist
