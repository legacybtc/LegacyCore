GO ?= go
PKG ?= ./...

.PHONY: build build-core daemon cli wallet-internal test vet frontend clean params \
	linux-amd64 linux-arm64 macos-amd64 macos-arm64 windows-amd64 \
	package-linux package-linux-arm64 package-macos-amd64 package-macos-arm64 package-windows \
	release-source release-verify scan-source verify-mainnet

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

linux-arm64:
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags "-s -w" -o dist/linux-arm64/legacycoind ./cmd/legacycoind
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags "-s -w" -o dist/linux-arm64/legacycoin-cli ./cmd/legacycoin-cli

macos-amd64:
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w" -o dist/macos-amd64/legacycoind ./cmd/legacycoind
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w" -o dist/macos-amd64/legacycoin-cli ./cmd/legacycoin-cli

macos-arm64:
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags "-s -w" -o dist/macos-arm64/legacycoind ./cmd/legacycoind
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags "-s -w" -o dist/macos-arm64/legacycoin-cli ./cmd/legacycoin-cli

windows-amd64:
	powershell.exe -ExecutionPolicy Bypass -File scripts/build-windows.ps1

package-linux:
	bash scripts/package-linux.sh v1.0.8 amd64

package-linux-arm64:
	bash scripts/package-linux.sh v1.0.8 arm64

package-macos-amd64:
	bash scripts/package-macos.sh v1.0.8 amd64

package-macos-arm64:
	bash scripts/package-macos.sh v1.0.8 arm64

package-windows:
	powershell.exe -ExecutionPolicy Bypass -File scripts/package-windows.ps1 -Version v1.0.8

release-source:
	powershell.exe -ExecutionPolicy Bypass -File scripts/release-source-archive.ps1 -Version v1.0.8 -OutputDir dist

release-verify:
	powershell.exe -ExecutionPolicy Bypass -File scripts/verify-release-assets.ps1 dist/*.zip dist/*.tar.gz

scan-source:
	powershell.exe -ExecutionPolicy Bypass -File scripts/scan-source-cleanliness.ps1 -Root .

verify-mainnet:
	powershell.exe -ExecutionPolicy Bypass -File scripts/verify-mainnet-identity.ps1 -Binary .\legacycoind.exe

clean:
	rm -rf legacycoind legacycoin-cli legacy-wallet-internal legacycoind.exe legacycoin-cli.exe legacy-wallet-internal.exe dist .gocache-local .gotmp-local .gocache-build .gotmp-build
