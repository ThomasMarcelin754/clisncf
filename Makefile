BINARY_NAME=sncf
BUILD_DIR=./cmd/sncf
INSTALL_DIR=/usr/local/bin
GOLANGCI := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: build install clean test vet lint vulncheck check dev

build:
	go build -ldflags '$(LDFLAGS)' -o $(BINARY_NAME) $(BUILD_DIR)

install: build
	install -m 755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)

test:
	go test ./... -race

vet:
	go vet ./...

lint:
	$(GOLANGCI) run ./...

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

check: vet lint test vulncheck build

dev:
	go run $(BUILD_DIR) $(ARGS)
