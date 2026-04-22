## Release Pull Request

## Summary

Describe what release is being prepared and why.

## Versioning

- Target version (SemVer):
- Tag to create:
- Changelog updated: yes/no

## Release Validation

- [ ] `make check`
- [ ] `make test-web`
- [ ] Artifacts built with `./build.sh` (or `FULL_MATRIX=1 ./build.sh`)
- [ ] `docs/release/checklist.md` completed
- [ ] Branch protection + required checks verified

## Security and Compliance

- [ ] `SECURITY.md` reporting instructions still valid
- [ ] No known medium+ vulnerabilities older than 60 days
- [ ] OpenSSF evidence docs reviewed (`BEST_PRACTICES.md`, release notes)

## Rollout Notes

- Deployment/rollout plan:
- Rollback plan:
