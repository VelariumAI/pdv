SHELL := /bin/sh

.PHONY: help check repo-check format test test-race verify-go ci

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make help        Show this help message' \
		'  make repo-check  Validate repository structure and required files' \
		'  make check       Run repository checks and Go checks when available' \
		'  make format      Format Go code when go.mod exists' \
		'  make test        Run go test ./... when go.mod exists' \
		'  make test-race   Run go test -race ./... when go.mod exists' \
		'  make ci          CI entrypoint'

repo-check:
	@test ! -d docs/superpowers || (echo 'legacy docs/superpowers directory must not exist'; exit 1)
	@test -f README.md || (echo 'README.md is required'; exit 1)
	@test -f CONTRIBUTING.md || (echo 'CONTRIBUTING.md is required'; exit 1)
	@test -f .gitignore || (echo '.gitignore is required'; exit 1)
	@test -f docs/README.md || (echo 'docs/README.md is required'; exit 1)
	@test -f docs/architecture/go-rewrite-design.md || (echo 'architecture design doc is required'; exit 1)
	@test -f docs/governance/agent-directives.md || (echo 'agent directives doc is required'; exit 1)
	@test -f docs/governance/agent-assignment-prompt.md || (echo 'agent assignment prompt is required'; exit 1)
	@test -f docs/plans/go-rewrite-implementation-plan.md || (echo 'implementation plan is required'; exit 1)
	@test -f docs/roadmap/tranches.md || (echo 'tranches doc is required'; exit 1)

verify-go:
	@if [ -f go.mod ]; then \
		echo 'go.mod detected; running Go quality checks'; \
		gofmt -w $$(find . -name '*.go' -type f); \
		go vet ./...; \
		go test ./... -count=1; \
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
		go test ./... -race -count=1; \
	else \
		echo 'go.mod not present; no Go race tests to run yet'; \
	fi

check: repo-check verify-go

ci: check
