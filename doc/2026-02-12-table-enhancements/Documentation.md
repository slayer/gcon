# Table Enhancements Documentation

## Summary

Added three features to the table component: column sorting, field-based filtering, and infinite scroll for the Objects view.

## Changes

### 1. Enhanced Column Definitions

Extended the `Column` struct in `internal/ui/components/table/table.go` with:
- `FilterKeys []string` - auto-derived from title for `field:value` filter syntax
- `Sortable bool` - marks columns that appear in the sort menu

All 9 list views migrated from `table.New()` to `table.NewWithColumns()` with appropriate column metadata.

### 2. Complex Filtering (`field:value` syntax)

The `/` filter now supports field-specific filtering in all views (except Objects which keeps simple text filter):

```
status:running           # Filter by status column
zone:us my-vm            # AND logic: zone contains "us" AND free text matches "my-vm"
machine_type:e2-micro    # Field keys derived from column titles
```

- Field keys are auto-derived: `"Machine Type"` -> `["machine_type", "machinetype", "type"]`
- Unknown field keys are treated as free text
- Case-insensitive matching throughout

**Files**: `table.go` (parseFilter, filterSpec, matchesFilterSpec), `filter_test.go`

### 3. Column Sorting

Press `S` to open a sort menu popup. Select a column by number/letter hotkey. Selecting the same column toggles sort direction. Sort indicator (`▲`/`▼`) shows in the column header.

- Numeric-aware comparison: `"100 GB"` sorts numerically, not as string
- Status icon stripping: leading `●`/symbols are ignored during comparison
- Sort persists across filter changes, resets on data refresh (`SetRows()`)

**Files**: `sortmenu/sortmenu.go`, `sortmenu/sortmenu_test.go`, `table.go` (SortBy, sortRows, compareValues, parseNumericValue), `sort_test.go`

### 4. Infinite Scroll (Objects View)

Replaced `n`/`p` page navigation with automatic infinite scroll:

- Page size increased from 100 to 200
- `NearBottomMsg` fires when cursor is within 5 rows of the table bottom
- New data appends seamlessly via `AppendRows()` (preserves cursor position)
- Status bar shows: `"200 items (scroll for more)"` / `"(loading more...)"` / `"(all loaded)"`
- Generation counter prevents stale responses from previous folder navigation

**Files**: `table.go` (NearBottomMsg, AppendRows, checkNearBottom), `near_bottom_test.go`, `objects.go`, `objects_test.go`

## New Files

| File | Description |
|------|-------------|
| `internal/ui/components/sortmenu/sortmenu.go` | Sort column selector popup |
| `internal/ui/components/sortmenu/sortmenu_test.go` | Sort menu tests |
| `internal/ui/components/table/filter_test.go` | Complex filtering tests |
| `internal/ui/components/table/sort_test.go` | Sorting tests |
| `internal/ui/components/table/near_bottom_test.go` | Infinite scroll tests |

## Modified Files

| File | Changes |
|------|---------|
| `internal/ui/components/table/table.go` | Column extensions, filtering, sorting, near-bottom detection |
| `internal/ui/views/objects.go` | Infinite scroll replaces pagination |
| `internal/ui/views/objects_test.go` | Updated tests for infinite scroll |
| `internal/ui/views/instances.go` | NewWithColumns migration, sort guard, help text |
| `internal/ui/views/disks.go` | Same |
| `internal/ui/views/snapshots.go` | Same |
| `internal/ui/views/images.go` | Same |
| `internal/ui/views/buckets.go` | Same |
| `internal/ui/views/networks.go` | Same |
| `internal/ui/views/firewalls.go` | Same |
| `internal/ui/views/projects.go` | Same |
| `.claude/rules/key-bindings.md` | Added S:sort, updated Objects view bindings |

## Testing

All features include comprehensive test coverage:
- `filter_test.go`: Field parsing, AND logic, case insensitivity, backward compatibility
- `sort_test.go`: String/numeric sorting, direction toggle, reset on SetRows, persist across filters
- `sortmenu_test.go`: Hotkey selection, toggle direction, close behavior
- `near_bottom_test.go`: Threshold detection, deduplication, AppendRows cursor preservation
- `objects_test.go`: Infinite scroll flow, generation mismatch handling, state management
