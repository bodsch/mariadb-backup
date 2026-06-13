BINARY      := mariadb-backup
PKG         := ./cmd/mariadb-backup
DIST        := dist

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS     := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

GOBUILD     := CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)"

.PHONY: all build build-linux-amd64 build-linux-arm64 build-all test vet fmt clean

all: build

build:
	$(GOBUILD) -o $(DIST)/$(BINARY) $(PKG)

build-linux-amd64:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(DIST)/$(BINARY)-linux-amd64 $(PKG)

build-linux-arm64:
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(DIST)/$(BINARY)-linux-arm64 $(PKG)

build-all: build-linux-amd64 build-linux-arm64

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -rf $(DIST)
