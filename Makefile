BINARY   := proxmox-mcp
MODULE   := github.com/trickyearlobe-ai/proxmox-mcp

# Derive version from git tags (e.g. v0.2.0 or v0.2.0-3-gabcdef)
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -X main.version=$(VERSION)

# ── Default: list targets ─────────────────────────────────────────────
.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk -F ':.*## ' '{printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Build & test ──────────────────────────────────────────────────────
.PHONY: build
build: ## Build the binary (version from git tags)
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

.PHONY: test
test: ## Run all tests
	go test ./... -count=1

.PHONY: test-verbose
test-verbose: ## Run all tests with verbose output
	go test ./... -v -count=1

.PHONY: lint
lint: ## Run go vet
	go vet ./...

.PHONY: clean
clean: ## Remove built binary
	rm -f $(BINARY)

# ── Install / Uninstall ──────────────────────────────────────────────
.PHONY: install
install: ## Install binary to GOPATH/bin and register with IDEs
	go install -ldflags "$(LDFLAGS)" .
	$(BINARY) --install

.PHONY: uninstall
uninstall: ## Unregister from IDEs
	$(BINARY) --uninstall

.PHONY: init
init: build ## Build and create ~/.proxmox.yaml template
	./$(BINARY) --init

# ── Versioning ────────────────────────────────────────────────────────
.PHONY: version
version: ## Show current version from git tags
	@echo $(VERSION)

.PHONY: bump-patch
bump-patch: ## Bump patch version (v0.2.0 → v0.2.1) and push tag
	@latest=$$(git tag -l 'v*' --sort=-v:refname | head -1); \
	if [ -z "$$latest" ]; then \
		echo "No existing tags. Creating v0.0.1"; \
		git tag -a v0.0.1 -m "v0.0.1"; \
		echo "Tagged v0.0.1"; \
	else \
		major=$$(echo $$latest | sed 's/^v//' | cut -d. -f1); \
		minor=$$(echo $$latest | sed 's/^v//' | cut -d. -f2); \
		patch=$$(echo $$latest | sed 's/^v//' | cut -d. -f3); \
		next="v$$major.$$minor.$$((patch + 1))"; \
		git tag -a $$next -m "$$next"; \
		echo "Tagged $$next (was $$latest)"; \
	fi

.PHONY: bump-minor
bump-minor: ## Bump minor version (v0.2.0 → v0.3.0) and push tag
	@latest=$$(git tag -l 'v*' --sort=-v:refname | head -1); \
	if [ -z "$$latest" ]; then \
		echo "No existing tags. Creating v0.1.0"; \
		git tag -a v0.1.0 -m "v0.1.0"; \
		echo "Tagged v0.1.0"; \
	else \
		major=$$(echo $$latest | sed 's/^v//' | cut -d. -f1); \
		minor=$$(echo $$latest | sed 's/^v//' | cut -d. -f2); \
		next="v$$major.$$((minor + 1)).0"; \
		git tag -a $$next -m "$$next"; \
		echo "Tagged $$next (was $$latest)"; \
	fi

.PHONY: bump-major
bump-major: ## Bump major version (v0.2.0 → v1.0.0) and push tag
	@latest=$$(git tag -l 'v*' --sort=-v:refname | head -1); \
	if [ -z "$$latest" ]; then \
		echo "No existing tags. Creating v1.0.0"; \
		git tag -a v1.0.0 -m "v1.0.0"; \
		echo "Tagged v1.0.0"; \
	else \
		major=$$(echo $$latest | sed 's/^v//' | cut -d. -f1); \
		next="v$$((major + 1)).0.0"; \
		git tag -a $$next -m "$$next"; \
		echo "Tagged $$next (was $$latest)"; \
	fi

.PHONY: push-tags
push-tags: ## Push all tags to origin
	git push origin --tags

.PHONY: release-patch
release-patch: bump-patch push-tags ## Bump patch version and push the tag

.PHONY: release-minor
release-minor: bump-minor push-tags ## Bump minor version and push the tag

.PHONY: release-major
release-major: bump-major push-tags ## Bump major version and push the tag

# ── Git hooks ─────────────────────────────────────────────────────────
.PHONY: setup-hooks
setup-hooks: ## Configure git to use the project's secret-detection hooks
	git config core.hooksPath githooks
	@echo "Git hooks path set to githooks/"
