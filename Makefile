# LabSyslog task runner. Unimplemented targets fail closed (exit 1).

GO ?= go
export GOTOOLCHAIN ?= local
export GOPROXY ?= https://proxy.golang.org,direct

GOVULNCHECK_MOD ?= golang.org/x/vuln/cmd/govulncheck@v1.1.4

.PHONY: help format lint generate verify-generated test test-race \
	test-fuzz-smoke test-parity test-config-compat test-docs test-container \
	security-scan test-changelog web-test web-build

help:
	@printf '%s\n' \
		'LabSyslog Make targets (Go 1.26; module github.com/hilather/go-lab-syslog)' \
		'  format              go fmt ./...' \
		'  lint                gofmt -l + go vet (no extra lint module)' \
		'  generate            JSON Schema, OpenAPI, error catalog, capabilities, metrics' \
		'  generate            JSON Schema, OpenAPI, error catalog, capabilities, MCP' \
		'  verify-generated    fail if generated API artifacts are stale' \
		'  test                go test ./...' \
		'  test-race           go test -race ./...' \
		'  test-fuzz-smoke     short go-fuzz of syslogwire.Parse + syslogframing.Next' \
		'  test-parity         REST/MCP capability parity goldens' \
		'  test-config-compat  valid/invalid YAML fixture suite' \
		'  test-docs           required documents, markdown links, required phrases' \
		'  test-container      scratch image, :1514 compose smoke, Bearer wait/reset' \
		'  security-scan       go vet + govulncheck (tool, not a product module)' \
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
	$(GO) test ./internal/syslogframing -count=1 -fuzz=FuzzNext -fuzztime=3s

test-parity:
	$(GO) test ./internal/capabilities ./internal/control/rest ./internal/control/mcp -count=1

test-container security-scan web-test web-build:
test-container:
	bash scripts/test-container.sh

security-scan:
	$(GO) vet ./...
	$(GO) run $(GOVULNCHECK_MOD) ./...

test-parity web-test web-build:
	@echo '$@: not implemented yet; placeholder fails closed' >&2
	@exit 1
