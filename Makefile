GO ?= go
PKG ?= ./...

.PHONY: build build-core daemon cli wallet-internal test vet frontend clean params \
	linux-amd64 windows-amd64 package-linux package-windows

build: build-core

build-core: daemon cli wallet-internal

daemon:
	$(GO) build -trimpath -o legacycoind ./cmd/legacycoind

cli:
	$(GO) build -trimpath -o legacycoin-cli ./cmd/legacycoin-cli

wallet-internal:
	$(GO) build -trimpath -o legacy-wallet-internal ./cmd/legacywallet

test:
	$(GO) test ./internal/p2p ./internal/rpc ./internal/wallet ./internal/mempool
	$(GO) test ./cmd/... ./internal/...

vet:
	$(GO) vet ./internal/p2p ./internal/rpc ./internal/wallet ./internal/mempool
	$(GO) vet ./cmd/... ./internal/...

frontend:
	cd cmd/legacywallet/frontend && npm install && npm run build

params:
	./legacycoind params

linux-amd64:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w" -o dist/linux-amd64/legacycoind ./cmd/legacycoind
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w" -o dist/linux-amd64/legacycoin-cli ./cmd/legacycoin-cli

windows-amd64:
	powershell.exe -ExecutionPolicy Bypass -File scripts/build-windows.ps1

package-linux:
	bash scripts/package-linux.sh

package-windows:
	powershell.exe -ExecutionPolicy Bypass -File scripts/package-windows.ps1

clean:
	rm -rf legacycoind legacycoin-cli legacy-wallet-internal legacycoind.exe legacycoin-cli.exe legacy-wallet-internal.exe dist .gocache-local .gotmp-local .gocache-build .gotmp-build
