BINARY     := tixotunnel
MODULE     := backhaul.local/ir_spoof
CMD        := ./cmd/backhaul
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE       ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w -X 'main.buildVersion=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.buildDate=$(DATE)'
DIST       := dist

.PHONY: build test vet fmt-check clean release release-amd64 release-arm64 checksums

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

test:
	go test ./...

vet:
	go vet ./...

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))" || \
		(echo "gofmt check failed; run gofmt -w" && gofmt -l $$(find . -name '*.go' -not -path './vendor/*') && exit 1)

clean:
	rm -rf bin $(DIST)

release-amd64:
	mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY) $(CMD)
	tar -czf $(DIST)/TixoTunnel-linux-amd64.tar.gz -C $(DIST) $(BINARY)
	rm -f $(DIST)/$(BINARY)

release-arm64:
	mkdir -p $(DIST)
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY) $(CMD)
	tar -czf $(DIST)/TixoTunnel-linux-arm64.tar.gz -C $(DIST) $(BINARY)
	rm -f $(DIST)/$(BINARY)

checksums:
	cd $(DIST) && sha256sum TixoTunnel-linux-amd64.tar.gz TixoTunnel-linux-arm64.tar.gz > SHA256SUMS

release: clean release-amd64 release-arm64 checksums
