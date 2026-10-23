# Demitable — the tasks
#
# The targets tech-stack §11 names: dev, test, corpus, build, lint.
# Plus the ones an agent reaches for every day: fmt, vet, check, run.
#
# `make help` lists them all.

SHELL := /bin/sh
.DEFAULT_GOAL := help

GO      ?= go
GOFLAGS ?=
BIN     ?= dist/demitable
PKG     ?= ./cmd/demitable
PORT    ?= 8080

# The frontend does not exist yet. When it does, `web/` appears and these
# targets start doing the frontend half as well.
WEB_DIR := web
ifneq ($(wildcard $(WEB_DIR)/package.json),)
HAVE_WEB := 1
endif

.PHONY: help
help: ## list the targets
	@echo "Demitable"
	@echo
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
	@echo
	@echo "  go test ./...          the Go suite"
	@echo "  go run ./cmd/...       the server"

# --- build ------------------------------------------------------------------

.PHONY: build
build: ## the full pipeline -> $(BIN)
	@mkdir -p dist
ifdef HAVE_WEB
	npm --prefix $(WEB_DIR) ci
	npm --prefix $(WEB_DIR) run build
	rm -rf internal/server/webdist
	cp -r $(WEB_DIR)/dist internal/server/webdist
endif
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -trimpath -ldflags "-s -w" -o $(BIN) $(PKG)
	@echo "built $(BIN)"

.PHONY: install
install: ## build and install into GOBIN
	$(GO) install $(GOFLAGS) $(PKG)

.PHONY: clean
clean: ## remove build output
	rm -rf dist
	rm -f $(WEB_DIR)/dist

# --- test -------------------------------------------------------------------

.PHONY: test
test: ## the Go suite
	$(GO) test $(GOFLAGS) ./...

.PHONY: test-race
test-race: ## the Go suite under the race detector
	$(GO) test $(GOFLAGS) -race ./...

.PHONY: cover
cover: ## the Go suite with a coverage summary
	$(GO) test $(GOFLAGS) -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: corpus
corpus: ## the golden-corpus suites (needs fixtures/)
	@if [ ! -d fixtures ]; then \
		echo "no corpus: fixtures/ is a transient build input and is not present."; \
		echo "see docs/corpus-provenance.md for how to obtain it."; \
		exit 1; \
	fi
	$(GO) test $(GOFLAGS) -run Corpus ./...

.PHONY: bench
bench: ## the Go benchmarks
	$(GO) test $(GOFLAGS) -run '^$$' -bench . ./...

# --- lint and format --------------------------------------------------------

.PHONY: lint
lint: ## gofmt check and go vet
	@out=$$(gofmt -l . 2>/dev/null); \
	if [ -n "$$out" ]; then \
		echo "these files are not gofmt'd:"; echo "$$out"; exit 1; \
	fi
	$(GO) vet ./...

.PHONY: fmt
fmt: ## gofmt the tree
	gofmt -w .

.PHONY: vet
vet: ## go vet
	$(GO) vet ./...

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: check
check: fmt vet test ## what CI runs

# --- run --------------------------------------------------------------------

.PHONY: run
run: ## run the server on :$(PORT)
	$(GO) run $(PKG) --port $(PORT)

.PHONY: dev
dev: ## the Go server and the Vite dev server (the frontend is not scaffolded yet)
ifdef HAVE_WEB
	npm --prefix $(WEB_DIR) run dev
else
	@echo "the frontend is not scaffolded yet: there is no $(WEB_DIR)/"
	@echo "running the Go server alone on :$(PORT)"
	$(GO) run $(PKG) --port $(PORT)
endif

# --- release ----------------------------------------------------------------

.PHONY: release
release: ## cross-compile the binary for every supported platform
	@mkdir -p dist
	@for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do \
		goos=$${target%/*}; goarch=$${target#*/}; \
		out=dist/demitable-$$goos-$$goarch; \
		if [ "$$goos" = "windows" ]; then out=$$out.exe; fi; \
		echo "  $$target"; \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch \
			$(GO) build $(GOFLAGS) -trimpath -ldflags "-s -w" -o $$out $(PKG) || exit 1; \
	done
