MODULE  := github.com/aallbrig/treemand/cmd
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    := $(shell date -u +%Y-%m-%d)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X '$(MODULE).Version=$(VERSION)' \
           -X '$(MODULE).Commit=$(COMMIT)' \
           -X '$(MODULE).BuildDate=$(DATE)'

.PHONY: build install test lint clean

## build: compile treemand binary into ./treemand
build:
	go build -ldflags "$(LDFLAGS)" -o treemand ./cli/treemand

## install: install treemand into GOPATH/bin (makes it globally available)
install:
	go install -ldflags "$(LDFLAGS)" ./cli/treemand

## test: run all tests
test:
	go test ./cli/treemand/...

## lint: run golangci-lint (requires golangci-lint on PATH)
lint:
	golangci-lint run ./cli/treemand/...

## clean: remove built binary
clean:
	rm -f treemand
