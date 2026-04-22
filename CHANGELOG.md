# Changelog

## 0.1.1 - 2026-04-22

- Added system-aware default `download_dir` resolution (Linux/macOS/Windows/Termux-aware).
- Added deterministic config tests covering OS-specific download directory fallback behavior.
- Cleaned local repository hygiene by removing scratch media/coverage artifacts and tightening `.gitignore`.
- Updated README configuration docs to reflect platform-aware default download destinations.

## 0.1.0 - 2026-04-17

- Implemented production queue lifecycle with persistence, retry backoff, startup requeue, and history writes.
- Added REST API server with queue/history/config/probe/system endpoints and uniform JSON responses.
- Expanded CLI with queue operations, probe, history, status, and `serve`.
- Added hardened tests and coverage gates; `internal/download` now meets the required minimum.
- Added cross-platform `build.sh` with version/date ldflags injection.
