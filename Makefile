# LabSyslog task runner. Unimplemented targets fail closed (exit 1).

GO ?= go
export GOTOOLCHAIN ?= local
export GOPROXY ?= https://proxy.golang.org,direct

GOVULNCHECK_MOD ?= golang.org/x/vuln/cmd/govulncheck@v1.1.4

.PHONY: help format lint generate verify-generated test test-race \
	test-fuzz-smoke test-parity test-config-compat test-docs test-container \
	security-scan test-changelog web-install web-test web-build web-embed \
	verify-web-dist

help:
	@printf '%s\n' \
		'LabSyslog Make targets (Go 1.26; module github.com/hilather/go-lab-syslog)' \
		'  format              go fmt ./...' \
		'  lint                gofmt -l + go vet (no extra lint module)' \
		'  generate            JSON Schema, OpenAPI, error catalog, capabilities, metrics, MCP' \
		'  verify-generated    fail if generated API artifacts are stale' \
		'  test                go test ./...' \
		'  test-race           go test -race ./...' \
		'  test-fuzz-smoke     short go-fuzz of syslogwire.Parse + syslogframing.Next (testdata/corpus)' \
		'  test-parity         REST/MCP capability parity goldens' \
		'  test-config-compat  valid/invalid YAML fixture suite' \
		'  test-docs           required documents, markdown links, NAT/userland-proxy/cap_add phrases' \
		'  test-container      scratch image, :1514 compose smoke, Bearer wait/reset' \
		'  security-scan       go vet + govulncheck (tool, not a product module)' \
		'  test-changelog      observable paths require a CHANGELOG.md entry' \
		'  web-test            vitest in web/ (Node 22.14.0)' \
		'  web-build           vite production build + embed into internal/web/dist' \
		'  verify-web-dist     fail if committed internal/web/dist is stale' \
		'All required 1.0 targets are implemented. Default CI includes import-fence.'

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

test-container:
	bash scripts/test-container.sh

security-scan:
	$(GO) vet ./...
	$(GO) run $(GOVULNCHECK_MOD) ./...

test-parity:
	$(GO) test ./internal/capabilities ./internal/control/rest ./internal/control/mcp -count=1

web-install:
	npm --prefix web ci

web-test:
	npm --prefix web test

web-build:
	npm --prefix web run build
	$(MAKE) web-embed

web-embed:
	@mkdir -p internal/web/dist
	@rm -rf internal/web/dist/assets
	@if [ -d web/dist ]; then cp -a web/dist/. internal/web/dist/; fi
	@echo "copied web/dist -> internal/web/dist"

# Scratch image embeds git's internal/web/dist (no Node stage). Fail if
# make web-build would rewrite the committed tree.
verify-web-dist:
	@if git diff --exit-code -- internal/web/dist >/dev/null && \
		[ -z "$$(git ls-files --others --exclude-standard -- internal/web/dist)" ]; then \
		echo "internal/web/dist matches HEAD"; \
	else \
		echo "internal/web/dist is stale versus make web-build; commit the embed" >&2; \
		git --no-pager diff -- internal/web/dist || true; \
		git ls-files --others --exclude-standard -- internal/web/dist; \
		exit 1; \
	fi
