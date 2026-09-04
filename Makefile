# LabSyslog task runner. Unimplemented targets fail closed (exit 1).

GO ?= go
export GOTOOLCHAIN ?= local
export GOPROXY ?= https://proxy.golang.org,direct

.PHONY: help format lint generate verify-generated test test-race \
	test-fuzz-smoke test-parity test-config-compat test-docs test-container \
	security-scan test-changelog web-test web-build

help:
	@printf '%s\n' \
		'LabSyslog Make targets (Go 1.26; module github.com/hilather/go-lab-syslog)' \
		'  format              go fmt ./...' \
		'  lint                gofmt -l + go vet (no extra lint module)' \
		'  generate            JSON Schema api/jsonschema/labsyslog.dev.v1alpha1.json' \
		'  verify-generated    fail if JSON Schema is stale' \
		'  test                go test ./...' \
		'  test-race           go test -race ./...' \
		'  test-fuzz-smoke     short go-fuzz of internal/syslogwire.Parse' \
		'  test-parity         placeholder (MCP-001)' \
		'  test-config-compat  valid/invalid YAML fixture suite' \
		'  test-docs           required documents, markdown links, required phrases' \
		'  test-container      placeholder (DEP-001)' \
		'  security-scan       placeholder (DEP-001)' \
		'  test-changelog      observable paths require a CHANGELOG.md entry' \
		'  web-test            placeholder (UI-001)' \
		'  web-build           placeholder (UI-001)' \
		'Placeholder targets exit 1. Default CI runs only implemented targets.'

format:
	$(GO) fmt ./...

lint:
	@unformatted=$$(gofmt -l cmd internal scripts); \
	if [ -n "$$unformatted" ]; then \
		printf '%s\n' 'gofmt needed:' $$unformatted; \
		exit 1; \
	fi
	$(GO) vet ./...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-docs:
	$(GO) run ./scripts/checkdocs

test-changelog:
	$(GO) run ./scripts/checkchangelog

generate:
	$(GO) run ./scripts/generatejsonschema

verify-generated:
	$(GO) run ./scripts/generatejsonschema --check

test-config-compat:
	$(GO) test ./internal/config ./internal/compiler ./internal/model ./internal/domainerr ./cmd/labsyslog ./scripts/generatejsonschema -count=1

test-fuzz-smoke:
	$(GO) test ./internal/syslogwire -count=1 -fuzz=FuzzParse -fuzztime=5s

test-parity test-container security-scan web-test web-build:
	@echo '$@: not implemented yet; placeholder fails closed' >&2
	@exit 1
