GO ?= go
PKG := ./...
BIN := bin/legacycoind

.PHONY: all build test race fuzz lint vuln sec clean release-snapshot

all: test build

build:
	$(GO) build -trimpath -ldflags='-s -w -buildid=' -o $(BIN) ./cmd/legacycoind

test:
	$(GO) test $(PKG)

race:
	$(GO) test -race $(PKG)

fuzz:
	$(GO) test ./internal/wire -run=^$$ -fuzz=Fuzz -fuzztime=30s
	$(GO) test ./internal/script -run=^$$ -fuzz=Fuzz -fuzztime=30s

lint:
	$(GO) vet $(PKG)
	staticcheck $(PKG)
	gosec ./...

vuln:
	govulncheck $(PKG)

sec: test race lint vuln

clean:
	rm -rf bin dist coverage.out coverage.html

release-snapshot: clean sec build
	mkdir -p dist
	tar --sort=name --mtime='UTC 2026-01-01' --owner=0 --group=0 --numeric-owner -czf dist/legacy-go-source.tar.gz \
		--exclude=.git --exclude=bin --exclude=dist --exclude=.gocache --exclude=.gotmp .
	sha256sum dist/legacy-go-source.tar.gz > dist/SHA256SUMS

.PHONY: mainnet-gate check-release-artifacts

mainnet-gate:
	sh ops/mainnet-gate.sh

check-release-artifacts:
	sh ops/check-release-artifacts.sh dist
