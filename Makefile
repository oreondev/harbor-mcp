VERSION ?= dev
LOCALBIN ?= $(shell pwd)/bin
GOLANGCI_LINT_VERSION ?= v2.13.1
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint

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

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Run golangci-lint.
	"$(GOLANGCI_LINT)" run

$(GOLANGCI_LINT):
	mkdir -p $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: help
help: ## Show the targets.
	@grep -E '^[a-z0-9-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "%-10s %s\n", $$1, $$2}'
