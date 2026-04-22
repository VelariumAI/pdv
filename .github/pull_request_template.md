## Summary

Describe the change and why it was made.

## Scope

- Tranche / task:
- Files or modules owned:
- User-facing surfaces affected:
- Release-impacting change: yes/no (if yes, also use `.github/PULL_REQUEST_TEMPLATE/release.md`)

## Verification

- [ ] `make repo-check`
- [ ] `go build ./...` if `go.mod` exists
- [ ] `go vet ./...` if `go.mod` exists
- [ ] `go test ./... -count=1` if `go.mod` exists
- [ ] `go test ./... -race -count=1` if `go.mod` exists

## Quality gate

- [ ] Full assigned scope is complete
- [ ] No placeholder logic or deferred wiring remains
- [ ] Docs and help text were updated where needed
- [ ] Remaining gaps: none

## References

- Release checklist: `docs/release/checklist.md`
- Branch protection baseline: `docs/release/branch-protection.md`
