GO ?= go
GOCACHE ?= $(CURDIR)/.gocache
GOMODCACHE ?= $(CURDIR)/.gomodcache

export GOCACHE
export GOMODCACHE

.PHONY: test unit integration clean cover

test: unit

unit:
	@echo "==> Running unit tests"
	$(GO) test ./...

integration:
	@echo "==> Running full test suite (integration)"
	GOTMUXCC_INTEGRATION=1 $(GO) test -tags integration ./...

cover:
	@echo "==> Generating coverage report"
	$(GO) test ./... -coverprofile=coverage.out
	@echo "Coverage summary:"
	$(GO) tool cover -func=coverage.out

clean:
	@echo "==> Cleaning caches"
	rm -rf $(GOCACHE) $(GOMODCACHE)
