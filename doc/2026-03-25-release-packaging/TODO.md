# Release Packaging

## Tasks

- [x] Add version variables and --version flag to cmd/gcon/main.go
- [x] Create .goreleaser.yaml
- [x] Rewrite .github/workflows/release.yml to use GoReleaser
- [x] Update .github/workflows/ci.yml Go version to 1.24
- [x] Update Makefile (version embedding, remove build-all)
- [x] Create install.sh
- [x] Test build locally with goreleaser check
- [x] Commit and create PR
- [x] Create homebrew-gcon repo
- [x] Test release pipeline with v0.7.0-rc1 tag
- [x] Address PR review comments
