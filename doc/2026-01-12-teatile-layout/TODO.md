# Teatile Layout Implementation

## Task Description

Replace manual height/newline management with Teatile library for automatic dimension handling to fix rendering glitches where the header disappears when content is too long.

## Root Cause

The current architecture uses `lipgloss.JoinHorizontal()` which requires both sidebar and content to have identical newline counts. Manual height tracking is error-prone and leads to rendering glitches.

## Implementation Plan

- [x] Add Teatile dependency to go.mod
- [x] Research Teatile API and patterns
- [x] Create tile-based layout manager in `internal/ui/layout/`
- [x] Refactor `internal/ui/app.go` to use tile hierarchy
- [x] Update views to receive dimensions from tiles
- [x] Test rendering on multiple terminals
- [x] Run full test suite and linter

## Tile Hierarchy Design

```
Root Tile (full terminal)
├── Header Tile (fixed height: 3 lines)
├── Content Tile (flex: remaining space)
│   ├── Sidebar Tile (fixed width: 25 chars)
│   └── Main Tile (flex: remaining width)
└── Footer Tile (fixed height: 1 line, help text)
```

## Key Technical Insights

1. **lipgloss.Height() only pads, doesn't truncate**: This was the root cause of the rendering issues. Content that exceeded its allocated height was not being truncated.

2. **lipgloss.MaxHeight() is required for truncation**: Using both `Height()` and `MaxHeight()` together ensures content is both padded (if short) and truncated (if long).

3. **Teatile for dimension calculation**: Teatile automatically calculates tile dimensions in a hierarchy, ensuring consistent sizing.

4. **Composition-level height enforcement**: Height constraints are applied at the final View() level, not in individual views. This allows views to render freely while the parent controls the final output dimensions.

## Success Criteria

- [x] No header disappearing on long content
- [x] No visual glitches on pagination
- [x] Consistent rendering across terminals
- [x] All tests pass
- [x] Linter passes
