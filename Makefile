.PHONY: build install test fmt vet clean run roles version help

BINARY  := drone
PKG     := ./cmd/drone
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.version=$(VERSION) -s -w

help:
	@echo "DevClaw kernel — Makefile targets"
	@echo "  make build     Build the drone binary into ./$(BINARY)"
	@echo "  make install   Install drone into \$$GOPATH/bin"
	@echo "  make test      Run unit tests"
	@echo "  make fmt       Format all Go files"
	@echo "  make vet       Run go vet"
	@echo "  make clean     Remove build artifacts"
	@echo "  make run       Build and run with --help"
	@echo "  make roles     Build and list roles"
	@echo "  make version   Print version"

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)
	@echo "Built ./$(BINARY) ($(VERSION))"

install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -f $(BINARY) $(BINARY).exe

run: build
	./$(BINARY) --help

roles: build
	./$(BINARY) roles

version: build
	./$(BINARY) version
