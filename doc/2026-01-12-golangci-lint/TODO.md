# Task: Add golangci-lint v2 to Project

## Description
Add golangci-lint v2.1.6 with version pinning to Makefile, GitHub Actions CI, and documentation.

## Implementation Plan

- [x] Create branch `2026-01-12-golangci-lint`
- [x] Update Makefile with version variable and install-lint target
- [x] Add lint job to `.github/workflows/ci.yml`
- [x] Update CLAUDE.md with install-lint command
- [x] Create `.golangci.yml` config file
- [x] Verify locally: `make install-lint && make lint`
- [x] Run tests: `make test`
- [x] Fix lint issues found by new linter
- [x] Create PR

## Files Modified
- `Makefile` - added GOLANGCI_LINT_VERSION variable and install-lint target
- `.github/workflows/ci.yml` - added lint job using golangci-lint-action@v7
- `CLAUDE.md` - documented install-lint command
- `.golangci.yml` - new config with common linters enabled
- `cmd/gcon/main.go` - fixed exitAfterDefer and errcheck issues
- `internal/config/gcloud.go` - fixed errcheck for file.Close()
- `internal/config/gcloud_test.go` - fixed errcheck for os.Remove()
- `internal/config/resolver_test.go` - fixed errcheck for os.Unsetenv()
- `internal/ui/views/instances.go` - fixed ineffassign issue
