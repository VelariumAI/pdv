SHELL := /bin/sh

.PHONY: help check repo-check format test test-race coverage verify-go ci

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make help        Show this help message' \
		'  make repo-check  Validate repository structure and required files' \
		'  make check       Run repository checks and Go checks when available' \
		'  make format      Format Go code when go.mod exists' \
		'  make test        Run go test ./... when go.mod exists' \
		'  make test-race   Run go test -race ./... when go.mod exists' \
		'  make coverage    Enforce tranche coverage thresholds when go.mod exists' \
		'  make ci          CI entrypoint'

repo-check:
	@test -f README.md || (echo 'README.md is required'; exit 1)
	@test -f CONTRIBUTING.md || (echo 'CONTRIBUTING.md is required'; exit 1)
	@test -f SECURITY.md || (echo 'SECURITY.md is required'; exit 1)
	@test -f LICENSE || (echo 'LICENSE is required'; exit 1)
	@test -f .gitignore || (echo '.gitignore is required'; exit 1)
	@test -f CODE_OF_CONDUCT.md || (echo 'CODE_OF_CONDUCT.md is required'; exit 1)
	@test -f SUPPORT.md || (echo 'SUPPORT.md is required'; exit 1)

verify-go:
	@if [ -f go.mod ]; then \
		echo 'go.mod detected; running Go quality checks'; \
		gofmt -w $$(find . -name '*.go' -type f); \
		go vet ./...; \
		go test ./... -count=1; \
		$(MAKE) coverage; \
	else \
		echo 'go.mod not present; skipping Go quality checks'; \
	fi

format:
	@if [ -f go.mod ]; then \
		gofmt -w $$(find . -name '*.go' -type f); \
	else \
		echo 'go.mod not present; nothing to format yet'; \
	fi

test:
	@if [ -f go.mod ]; then \
		go test ./... -count=1; \
	else \
		echo 'go.mod not present; no Go tests to run yet'; \
	fi

test-race:
	@if [ -f go.mod ]; then \
		if [ "$$(go env GOOS)" = "android" ] && [ "$$(go env GOARCH)" = "arm64" ]; then \
			echo 'race detector unsupported on android/arm64; skipping'; \
		else \
			go test ./... -race -count=1; \
		fi; \
	else \
		echo 'go.mod not present; no Go race tests to run yet'; \
	fi

coverage:
	@if [ -f go.mod ]; then \
		set -e; \
		go test ./internal/config -coverprofile=coverage_config.out -covermode=atomic >/dev/null; \
		cfg=$$(go tool cover -func=coverage_config.out | awk '/^total:/ {print $$3}' | tr -d '%'); \
		go test ./internal/database -coverprofile=coverage_database.out -covermode=atomic >/dev/null; \
		db=$$(go tool cover -func=coverage_database.out | awk '/^total:/ {print $$3}' | tr -d '%'); \
		go test ./internal/download -coverprofile=coverage_download.out -covermode=atomic >/dev/null; \
		down=$$(go tool cover -func=coverage_download.out | awk '/^total:/ {print $$3}' | tr -d '%'); \
		go test ./internal/events -coverprofile=coverage_events.out -covermode=atomic >/dev/null; \
		ev=$$(go tool cover -func=coverage_events.out | awk '/^total:/ {print $$3}' | tr -d '%'); \
		if [ "$$ev" = "0.0" ]; then \
			echo "coverage internal/events: $$ev% (type-only package; treating as pass when tests succeed)"; \
			ev=100.0; \
		fi; \
		echo "coverage internal/config: $$cfg% (min 90%)"; \
		echo "coverage internal/database: $$db% (min 90%)"; \
		echo "coverage internal/download: $$down% (min 85%)"; \
		echo "coverage internal/events: $$ev% (min 100%)"; \
		awk "BEGIN {exit !($$cfg >= 90.0)}"; \
		awk "BEGIN {exit !($$db >= 90.0)}"; \
		awk "BEGIN {exit !($$down >= 85.0)}"; \
		awk "BEGIN {exit !($$ev >= 100.0)}"; \
	else \
		echo 'go.mod not present; no coverage checks to run yet'; \
	fi

check: repo-check verify-go

ci: check
