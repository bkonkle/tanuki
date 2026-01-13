# Tanuki - Multi-agent orchestration for Claude Code

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags "-X github.com/bkonkle/tanuki/internal/cli.version=$(VERSION) \
	-X github.com/bkonkle/tanuki/internal/cli.commit=$(COMMIT) \
	-X github.com/bkonkle/tanuki/internal/cli.date=$(DATE)"

BINARY := tanuki
INSTALL_PATH := $(GOPATH)/bin
ifeq ($(INSTALL_PATH),/bin)
	INSTALL_PATH := $(HOME)/go/bin
endif

.PHONY: all build install test clean lint fmt vet

all: build

## build: Build the tanuki binary
build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/tanuki

## install: Install tanuki to GOPATH/bin
install:
	go install $(LDFLAGS) ./cmd/tanuki

## test: Run all tests
test:
	go test -v -race ./...

## test-coverage: Run tests with coverage report
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

## clean: Remove build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## fmt: Format code
fmt:
	go fmt ./...

## vet: Run go vet
vet:
	go vet ./...

## tidy: Tidy go modules
tidy:
	go mod tidy

## help: Show this help
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
