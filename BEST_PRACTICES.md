# OpenSSF Best Practices Passing Alignment

This document maps PDV evidence to the OpenSSF Best Practices "Passing" criteria:
`https://www.bestpractices.dev/en/projects/1/passing`

It is intended to make badge submission and audit review straightforward.

## Core Evidence Map

- Project description and usage:
  - `README.md`
- Contribution and acceptable change requirements:
  - `CONTRIBUTING.md`
  - `.github/pull_request_template.md`
- FLOSS license and license location:
  - `LICENSE`
- Basic and interface documentation:
  - `README.md`
  - `docs/architecture/go-rewrite-design.md`
- Searchable public discussion/reporting:
  - GitHub Issues and Pull Requests
  - `.github/ISSUE_TEMPLATE/*`
- Vulnerability reporting process:
  - `SECURITY.md`
- Public version control and change history:
  - Git repository + commit history + tags
- Unique release versions and release notes:
  - `cmd/pdv/version.go`
  - `CHANGELOG.md`
  - git tags (`v*.*.*`)
- Build and automated testing:
  - `Makefile`
  - `.github/workflows/ci.yml`
  - `.github/workflows/release.yml`
- Test policy and evidence:
  - `CONTRIBUTING.md`
  - tests in `internal/*/*_test.go`
- Static analysis before major release:
  - `.github/workflows/ci.yml` (`go vet`, `staticcheck`)

## Operational Notes

- Vulnerability response targets are documented in `SECURITY.md`:
  - Acknowledge within 72 hours.
  - Initial triage response within 14 days.
- Release workflow runs CI quality gates before release artifact generation.
- Release artifacts are delivered over HTTPS through GitHub releases.

## Submission Checklist

Before submitting/updating badge answers:

1. Verify all linked evidence URLs are current and public.
2. Confirm latest CI runs are passing on default branch.
3. Confirm latest release uses SemVer tag + changelog entry.
4. Confirm `SECURITY.md` process is still accurate and monitored.
