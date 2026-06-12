BINARY := pfx2as-neighbor
INSTALL_BIN := /usr/local/bin/$(BINARY)
INSTALL_CFG := /etc/$(BINARY)
SERVICE_FILE := /etc/systemd/system/$(BINARY).service
VERSION := $(shell cat VERSION)

.PHONY: build build-deb test fmt fmt-check hooks

build:
	GOOS=linux GOARCH=amd64 go build -o ./build/$(BINARY) ./cmd/$(BINARY)

build-deb: build
	VERSION=$(VERSION) nfpm package --packager deb --target ./dist/

test:
	go test -v ./...

fmt:
	gofmt -w .

fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Not gofmt-clean:"; echo "$$unformatted"; exit 1; \
	fi

hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks installed (core.hooksPath=.githooks)"
