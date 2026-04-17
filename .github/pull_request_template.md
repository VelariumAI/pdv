## Summary

Describe the change and why it was made.

## Scope

- Tranche / task:
- Files or modules owned:
- User-facing surfaces affected:

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
