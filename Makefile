.PHONY: all build dev test lint cover clean install

PKG := github.com/jadmadi/swapaudit/cmd/swapaudit
VERSION_PKG := github.com/jadmadi/swapaudit/internal/version

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

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

install: build
	mkdir -p $(HOME)/.local/bin
	cp $(BIN) $(HOME)/.local/bin/$(BIN)
