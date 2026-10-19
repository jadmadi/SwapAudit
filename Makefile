.PHONY: all build dev test lint cover clean install

PKG := github.com/jadmadi/swapaudit/cmd/swapaudit
VERSION_PKG := github.com/jadmadi/swapaudit/internal/version

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell if [ -n "$${SOURCE_DATE_EPOCH:-}" ]; then date -u -d "@$${SOURCE_DATE_EPOCH}" +"%Y-%m-%dT%H:%M:%SZ"; else date -u +"%Y-%m-%dT%H:%M:%SZ"; fi)

LDFLAGS := -s -w \
	-X $(VERSION_PKG).Version=$(VERSION) \
	-X $(VERSION_PKG).Commit=$(COMMIT) \
	-X $(VERSION_PKG).Date=$(DATE)

BIN := swapaudit

all: test build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN) $(PKG)

dev:
	go build -o $(BIN) $(PKG)

test:
	go test -v -race ./...

lint:
	gofmt -l .
	test -z "$$(gofmt -l .)"
	go vet ./...
	which golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || true

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -f $(BIN) coverage.out coverage.html
	rm -rf dist/ build/

PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

install:
	./scripts/build --release --install="$(BINDIR)"
