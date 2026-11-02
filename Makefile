MODULE  := github.com/cmerk2021/panely
BINARY  := panely
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X $(MODULE)/internal/appinfo.Version=$(VERSION) \
	-X $(MODULE)/internal/appinfo.Commit=$(COMMIT) \
	-X $(MODULE)/internal/appinfo.Date=$(DATE)

.PHONY: build run test lint fmt tidy snapshot clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/panely

run: build
	./bin/$(BINARY)

test:
	go test -race ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .

tidy:
	go mod tidy

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf bin dist
