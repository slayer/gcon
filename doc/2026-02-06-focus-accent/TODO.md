# Visual Focus Indicator for Active Regions

## Task Description
Add a left accent bar (`│`) in blue (`#4285F4`) on the focused region only, plus a region badge in help text.

## Implementation Plan
- [x] Step 1: Create `internal/ui/focus/accent.go` — `RenderAccent()` utility
- [x] Step 2: Create `internal/ui/focus/accent_test.go` — tests
- [x] Step 3: Add `FormatRegionBadge()` to `internal/ui/focus/help.go`
- [x] Step 4: Add tests for `FormatRegionBadge()` to help_test.go
- [x] Step 5: Update `instance_details.go` — accent on tabs + viewport, region badge in help
- [x] Step 6: Update `object_details.go` — accent on tabs + viewport, region badge in help
- [x] Step 7: Update `snapshot_details.go` — accent on viewport, region badge in help
- [x] Step 8: Build, test, lint
