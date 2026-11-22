# booster's build: the page (frontend/, Vite) embedded in one Go binary.
#
# Wails' default GTK4 backend needs GTK 4.10+, and Ubuntu 22.04 has 4.6, so
# the window builds on GTK3 (WebKitGTK 4.1, libwebkit2gtk-4.1-dev) by
# default; `make TAGS=` builds the GTK4 backend.
TAGS ?= gtk3
BIN := bin/booster

.PHONY: build page check clean

build: page
	go build -tags '$(TAGS),production' -trimpath -o $(BIN) .

# The Go build embeds frontend/dist, so vet and test need the page too.
page: frontend/node_modules/.package-lock.json
	cd frontend && npm run build

frontend/node_modules/.package-lock.json: frontend/package-lock.json
	cd frontend && npm ci --no-audit --no-fund

check: page
	@out=$$(gofmt -l main.go internal); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	go vet -tags '$(TAGS)' ./...
	go test -race -tags '$(TAGS)' ./...

clean:
	rm -rf bin frontend/dist
