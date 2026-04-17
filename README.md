# PDV

PDV is a Go rewrite of Personal Data Vault: a self-hosted download manager with a queue, REST API, CLI, and terminal UI.

The repository is currently in project setup and implementation-planning stage. The codebase skeleton exists, and the governing documents define the production standards required for all future implementation work.

The intended public repository home is `github.com/velariumai/pdv`.

## Repository Layout

- `cmd/` — entrypoints
- `internal/` — application packages
- `pkg/` — shared public data types
- `docs/` — architecture, governance, plans, and roadmap documents

## Core Docs

- [docs/README.md](docs/README.md)
- [Architecture Design](docs/architecture/go-rewrite-design.md)
- [Agent Directives](docs/governance/agent-directives.md)
- [Agent Assignment Prompt](docs/governance/agent-assignment-prompt.md)
- [Implementation Plan](docs/plans/go-rewrite-implementation-plan.md)
- [Tranches](docs/roadmap/tranches.md)

## Standards

Implementation work in this repository is governed by the documents under `docs/`. Production-ready delivery, full-scope ownership, and explicit verification are mandatory.

## Repository Operations

- `make help` shows the available repository tasks
- `make repo-check` validates repo structure and required documentation
- `make check` runs repository checks and Go checks when `go.mod` exists
- GitHub Actions enforces repository hygiene immediately and Go quality gates once the module is created
