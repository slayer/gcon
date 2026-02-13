# Table Enhancements: Sorting, Complex Filtering, Infinite Scroll

## Task Description
Add three features to the table component:
1. Column sorting via popup sort menu (all list views except Objects and Projects)
2. Field-based filtering with `field:value` syntax (all list views except Objects)
3. Infinite scroll for Objects view (replaces `n`/`p` pagination)

## Implementation Plan

### Step 1: Extend Column struct
- [x] Add `FilterKeys []string` and `Sortable bool` to `Column` struct
- [x] Add `deriveFilterKeys()` helper
- [x] Update `NewWithColumns()` to auto-derive filter keys

### Step 2: Migrate views to NewWithColumns()
- [x] instances.go
- [x] disks.go
- [x] snapshots.go
- [x] images.go
- [x] buckets.go
- [x] networks.go
- [x] firewalls.go
- [x] projects.go
- [x] objects.go

### Step 3: Complex filtering
- [x] Add filterSpec type and parseFilter()
- [x] Modify applyFilter() for field:value syntax
- [x] Update filter placeholder
- [x] Write filter_test.go

### Step 4: Sort menu component
- [x] Create sortmenu package
- [x] Message types, styles, View, Update
- [x] sortmenu_test.go

### Step 5: Sorting in table
- [x] Add sort state to Model
- [x] Add S key binding
- [x] SortBy(), ClearSort(), sortRows()
- [x] Sort indicator in headers
- [x] Sort menu overlay in View()
- [x] sort_test.go

### Step 6: Sort guard in views
- [x] Add IsSortMenuOpen() check to all views
- [x] Update help text to include S:sort
- [x] Remove SSH key binding from instances.go

### Step 7: Infinite scroll
- [x] Add NearBottomMsg, threshold, AppendRows to table
- [x] Rewrite objects.go pagination
- [x] near_bottom_test.go
- [x] objects_test.go infinite scroll tests

### Step 8: Documentation
- [x] Update key-bindings.md (add S:sort, remove n/p from objects, add field:value hint)
- [x] Update TODO.md
- [x] Create Documentation.md
- [x] Run tests and linter
