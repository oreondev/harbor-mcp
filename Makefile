VERSION ?= dev
LOCALBIN ?= $(shell pwd)/bin
GOLANGCI_LINT_VERSION ?= v2.13.1
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
MCP_PUBLISHER_VERSION ?= v1.8.7
MCP_PUBLISHER = $(LOCALBIN)/publisher
IMG ?= harbor-mcp:dev
DIST_PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: all
all: build

.PHONY: build
build: ## Build bin/harbor-mcp.
	go build -ldflags "-X main.version=$(VERSION)" -o $(LOCALBIN)/harbor-mcp ./cmd/harbor-mcp

.PHONY: test
test: ## Unit tests against the fake Harbor.
	go vet ./...
	go test ./... -coverprofile cover.out

.PHONY: test-e2e
test-e2e: ## Run the binary against the Harbor in HARBOR_URL, HARBOR_USERNAME, HARBOR_PASSWORD.
	go test -tags=e2e ./test/e2e/ -v -count=1

.PHONY: docker-build
docker-build: ## Build the image IMG for the local platform.
	docker build --build-arg VERSION=$(VERSION) -t $(IMG) .

.PHONY: dist
dist: ## Build release archives for DIST_PLATFORMS and dist/checksums.txt.
	rm -rf dist && mkdir -p dist
	set -e; for p in $(DIST_PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; name=harbor-mcp_$(VERSION)_$${os}_$${arch}; ext=; \
		[ "$$os" = windows ] && ext=.exe; \
		mkdir -p dist/$$name; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" \
			-o dist/$$name/harbor-mcp$$ext ./cmd/harbor-mcp; \
		cp LICENSE README.md dist/$$name/; \
		if [ "$$os" = windows ]; then (cd dist && zip -qr $$name.zip $$name); \
		else tar -C dist -czf dist/$$name.tar.gz $$name; fi; \
		rm -rf dist/$$name; \
	done
	cd dist && sha256sum *.tar.gz *.zip > checksums.txt

.PHONY: validate-server-json
validate-server-json: $(MCP_PUBLISHER) ## Validate server.json with mcp-publisher (asks the registry).
	"$(MCP_PUBLISHER)" validate server.json

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Run golangci-lint.
	"$(GOLANGCI_LINT)" run

.PHONY: lint-config
lint-config: $(GOLANGCI_LINT) ## Verify the golangci-lint configuration.
	"$(GOLANGCI_LINT)" config verify

$(MCP_PUBLISHER):
	mkdir -p $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/modelcontextprotocol/registry/cmd/publisher@$(MCP_PUBLISHER_VERSION)

$(GOLANGCI_LINT):
	mkdir -p $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: help
help: ## Show the targets.
	@grep -E '^[a-z0-9-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "%-22s %s\n", $$1, $$2}'
