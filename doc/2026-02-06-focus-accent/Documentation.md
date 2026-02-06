# Visual Focus Indicator for Active Regions

## Summary

Added a left accent bar (`│`) in blue (`#4285F4`) on the focused region and a region badge in help text, so users can tell at a glance which region has focus.

## Changes

### New Files

- **`internal/ui/focus/accent.go`** — `RenderAccent(content, focused)` prepends a blue `│` to each line when focused, or a space when unfocused (for alignment).
- **`internal/ui/focus/accent_test.go`** — Tests for focused/unfocused, single/multi-line, empty content.

### Modified Files

- **`internal/ui/focus/help.go`** — Added `FormatRegionBadge(region)` returning a styled `▸ Label` badge.
- **`internal/ui/focus/help_test.go`** — Tests for `FormatRegionBadge` with nil, empty label, and various labels.
- **`internal/ui/views/instance_details.go`** — Tab bar and viewport wrapped with `RenderAccent`; help text shows region badge.
- **`internal/ui/views/object_details.go`** — Tab bar and viewport wrapped with `RenderAccent`; help text shows region badge.
- **`internal/ui/views/snapshot_details.go`** — Viewport wrapped with `RenderAccent`; help text shows region badge.

## Visual Result

When viewport is focused:
```
  [Details]  Observability
 │ General Information
 │   Name: instance-1
  ▸ Content • j/k: scroll • tab: next region • .: actions
```

When tabs are focused:
```
 │[Details]  Observability
  General Information
    Name: instance-1
  ▸ Tabs • h/l: switch tab • 1-9: go to tab • tab: next region • .: actions
```

## Code Review Findings

**Layout overflow risk (fixed):** The accent bar adds 1 character width to each line, but viewports were sized to the full content width. This caused focused content to overflow by 1 character on narrow terminals. Fixed by computing `viewportWidth = width - 1` in all three detail views' `applySize()` methods.

## Testing

- `make build` — passes
- `make test` — all tests pass
- `make lint` — 0 issues
- Manual testing recommended: verify accent bar doesn't cause overflow on narrow terminals (80 columns)
