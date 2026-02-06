# Reduce UI Code Duplication and Unify Patterns

## Task Description
Eliminate code duplication across views by extracting shared patterns into reusable helpers and base types.

## Implementation Plan

- [x] Phase 1: Spinner Factory Function — `NewGCPSpinner()` in spinner.go, replace 20+ inline creations
- [x] Phase 2: Shared `renderLoading`/`renderSaving` helpers in views/helpers.go
- [x] Phase 3: Form sizing constants (`formWidthPadding`, `formHeightPadding`)
- [x] Phase 4: `RenderInlineError` in error_display.go with tests
- [x] Phase 5: `TableClickDelegate` embeddable struct for 7 list views
- [x] Phase 6: `CreateViewBase` for disk/snapshot/image creation views

## Verification
- `make test` — all pass
- `make lint` — 0 issues

## Results
- 32 files changed, 321 insertions(+), 836 deletions(-)
- Net reduction: ~515 lines of duplicated code
- New shared components: `NewGCPSpinner`, `renderLoading`, `renderSaving`, `RenderInlineError`, `TableClickDelegate`, `CreateViewBase`
