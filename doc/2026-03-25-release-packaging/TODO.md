# Release Packaging

## Tasks

- [ ] Add version variables and --version flag to cmd/gcon/main.go
- [ ] Create .goreleaser.yaml
- [ ] Rewrite .github/workflows/release.yml to use GoReleaser
- [ ] Update .github/workflows/ci.yml Go version to 1.24
- [ ] Update Makefile (version embedding, remove build-all)
- [ ] Create install.sh
- [ ] Test build locally with goreleaser check
- [ ] Commit and create PR
