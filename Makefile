.PHONY: build web test e2e run dev clean bench corpus install release licenses
VERSION ?= $(shell cat VERSION 2>/dev/null || echo 0.0.0-dev)

# The UI is built straight into internal/ui/dist, which the Go binary embeds.
build: web
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/docent ./cmd/docent

web:
	cd web && pnpm install --frozen-lockfile && pnpm build

# Everything that does not need a browser, then the browser tests against the real binary.
test:
	go vet -composites=false ./...
	go test -race ./...
	cd web && pnpm check && pnpm test

# Put the binary on your PATH (default ~/.local/bin; set PREFIX to change it).
PREFIX ?= $(HOME)/.local
install: build
	install -Dm755 bin/docent $(PREFIX)/bin/docent
	install -Dm644 packaging/docent.desktop $(PREFIX)/share/applications/docent.desktop
	install -Dm644 packaging/docent.svg $(PREFIX)/share/icons/hicolor/scalable/apps/docent.svg
	@echo installed $(PREFIX)/bin/docent, with a launcher entry and icon

# The licences of the Go modules compiled in (the npm list is written by `pnpm build`, see web/licenses.ts).
licenses:
	./scripts/go-licenses.sh

# Linux release archives and checksums in dist/ (the UI is built once; Go cross-compiles).
release: web licenses
	rm -rf dist && mkdir -p dist
	for arch in amd64 arm64; do \
	  d=dist/docent-$(VERSION)-linux-$$arch; mkdir -p $$d; \
	  CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $$d/docent ./cmd/docent; \
	  cp README.md LICENSE NOTICE.md CHANGELOG.md VERSION $$d/; cp -r licenses $$d/; cp internal/help/guide.md $$d/GUIDE.md; cp -r packaging $$d/; cp -r docs $$d/docs; \
	  tar -C dist -czf $$d.tar.gz docent-$(VERSION)-linux-$$arch; rm -rf $$d; \
	done
	cd dist && sha256sum *.tar.gz > SHA256SUMS && cat SHA256SUMS

e2e: build
	cd web && pnpm exec playwright test

run: build
	./bin/docent serve

# UI development: `docent serve --no-open --addr 127.0.0.1:8787` in one terminal, then this (Vite proxies /_api to it, with hot reload).
dev:
	cd web && pnpm dev

clean:
	rm -rf bin dist internal/ui/dist/assets web/test-results

bench:
	./scripts/bench.sh $(CORPUS) 10000

corpus:
	DOCENT_CORPUS=$(CORPUS) go test ./internal/render -run Corpus -v
