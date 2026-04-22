# PDV

[![CI](https://github.com/velariumai/pdv/actions/workflows/ci.yml/badge.svg)](https://github.com/velariumai/pdv/actions/workflows/ci.yml)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/0/badge)](https://www.bestpractices.dev/projects/0)

PDV is a self-hosted download manager with a persistent queue, retry/backoff worker engine, REST API, and CLI.

Note: OpenSSF badge link currently uses a placeholder project id (`0`) until the PDV badge entry is created.

## Project Links

- Source: `https://github.com/velariumai/pdv`
- Issues (bugs/enhancements): `https://github.com/velariumai/pdv/issues`
- Support: [SUPPORT.md](SUPPORT.md)
- Security policy: [SECURITY.md](SECURITY.md)
- Code of conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Contribution guide: [CONTRIBUTING.md](CONTRIBUTING.md)

## Quick Start

```bash
go build -o pdv ./cmd/pdv
./pdv --help
./pdv add "https://example.com/video"
./pdv list
./pdv history 20
./pdv serve --api-host 0.0.0.0 --api-port 8787
```

## Web GUI

- GUI assets live in `web/` (`web/index.html`, logos, favicon).
- `pdv serve` automatically serves the GUI at `/` when `index.html` is present in `./web` (or the config directory as fallback).
- The GUI API base defaults to same-origin (`<server>/api/v1`) and falls back to `http://localhost:8787/api/v1` when opened from a non-HTTP origin.

## CLI Commands

- `pdv add <url> [--quality --format --template --category --playlist]`
- `pdv probe <url>`
- `pdv list [--status]`
- `pdv pause <id>` / `pdv resume <id>` / `pdv retry <id>` / `pdv cancel <id>`
- `pdv history [limit] [--status]`
- `pdv status`
- `pdv serve [--api-host --api-port]`
- `pdv get [key] [--all]`
- `pdv set <key> <value>`
- `pdv config validate`
- `pdv --version`

## REST API

Base path: `/api/v1`

- Queue
- `GET /queue`
- `POST /queue`
- `GET /queue/:id`
- `DELETE /queue/:id`
- `POST /queue/:id/pause`
- `POST /queue/:id/resume`
- `POST /queue/:id/retry`
- `POST /queue/pause`
- `POST /queue/resume`
- `DELETE /queue`
- History
- `GET /history`
- `GET /history/:id`
- `DELETE /history/:id`
- `DELETE /history`
- Config
- `GET /config`
- `PUT /config`
- `GET /config/:key`
- `PUT /config/:key`
- `GET /config/export`
- `POST /config/import`
- System
- `POST /probe`
- `GET /status`
- `GET /health`
- `GET /ready`
- `GET /logs`
- `POST /shutdown`

All responses use:

```json
{"success": true, "data": {}, "error": ""}
```

## Configuration

Common keys:

- `max_concurrent_queue`
- `download_dir`
- `output_template`
- `output_template_playlist`
- `default_quality`
- `auto_categorize`
- `api_host`
- `api_port`
- `retries`
- `api_token` (optional; protects mutating API endpoints)
- `cors_allowed_origins` (comma-separated allowlist; set `*` explicitly to allow all)

Default `download_dir` is system-aware:
- Linux/macOS: `~/Downloads`
- Windows: `%USERPROFILE%/Downloads`
- Termux/Android: shared download folder (`/storage/emulated/0/Download`) with safe fallbacks

Use:

```bash
./pdv get --all
./pdv set retries 5
```

When `api_token` is set, mutating API routes (`POST/PUT/DELETE`) require either:

- `Authorization: Bearer <token>`
- `X-API-Token: <token>`

## Build and Verification

- `make check` runs formatting, vet, tests, and coverage gates.
- `make test-web` runs a smoke test against `pdv serve` (`GET /` + `GET /api/v1/status`).
- `./build.sh` emits the local Termux ARM64 artifact to `dist/`.
- `FULL_MATRIX=1 ./build.sh` attempts the full cross-platform matrix.
- `pdv --version` prints version/build date (supports `-ldflags` injection via `build.sh`).

## Versioning and Releases

- PDV uses SemVer (`MAJOR.MINOR.PATCH`).
- Git tags use the `v` prefix (for example, `v0.1.1`).
- User-facing changes are summarized in [CHANGELOG.md](CHANGELOG.md).

## Platform Notes

- Termux ARM64 is supported (`pdv-termux-arm64` artifact).
- Linux/macOS/Windows builds are produced by `build.sh`.

## Project Docs

- [docs/README.md](docs/README.md)
- [docs/architecture/go-rewrite-design.md](docs/architecture/go-rewrite-design.md)
- [docs/governance/agent-directives.md](docs/governance/agent-directives.md)
- [docs/roadmap/tranches.md](docs/roadmap/tranches.md)
- [docs/release/checklist.md](docs/release/checklist.md)
- [docs/release/branch-protection.md](docs/release/branch-protection.md)
