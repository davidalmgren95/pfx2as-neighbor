BINARY := pfx2as-neighbor
INSTALL_BIN := /usr/local/bin/$(BINARY)
INSTALL_CFG := /etc/$(BINARY)
SERVICE_FILE := /etc/systemd/system/$(BINARY).service
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build build-deb

build:
	GOOS=linux GOARCH=amd64 go build -o ./build/$(BINARY) ./cmd/$(BINARY)

build-deb: build
	VERSION=$(VERSION) nfpm package --packager deb --target ./dist/
