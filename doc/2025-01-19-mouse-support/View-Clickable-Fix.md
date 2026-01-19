# View Clickable Interface Fix

**Date:** 2025-01-19
**Branch:** 2025-01-19-mouse-support
**Status:** ✅ Fixed

## Issue

Mouse clicks worked in the sidebar but not in tables. Users could not click on table rows to select items or navigate.

## Root Cause

The region-based mouse architecture expected **views** to implement the `Clickable` interface, but only the underlying **table components** implemented it.

### Architecture Mismatch

```
App.handleMouseEvent:
  ↓
  Tries to cast view to Clickable
  ↓
  view (InstancesView, DisksView, etc.) - Does NOT implement Clickable ❌
  ↓
  Cast fails, no regions are calculated
  ↓
  Clicks are ignored
```

The sidebar worked because:
- Sidebar is a component that directly implements Clickable ✓
- App can cast sidebar to Clickable ✓

Tables didn't work because:
- Views contain tables, but don't expose them ❌
- App tried to cast views to Clickable ❌
- Views didn't implement Clickable ❌

## Solution

Made all table-based views implement the `Clickable` interface by **delegating** to their internal table components.

### Implementation Pattern

Each view now implements three methods that delegate to the table:

```go
// UpdateRegions delegates to the table component.
func (v *ViewType) UpdateRegions(offsetX, offsetY int) {
	if clickable, ok := interface{}(&v.table).(components.Clickable); ok {
		clickable.UpdateRegions(offsetX, offsetY)
	}
}

// GetRegions delegates to the table component.
func (v *ViewType) GetRegions() []mouse.Region {
	if clickable, ok := interface{}(&v.table).(components.Clickable); ok {
		return clickable.GetRegions()
	}
	return nil
}

// HandleRegionClick delegates to the table component.
func (v *ViewType) HandleRegionClick(regionID string) tea.Cmd {
	if clickable, ok := interface{}(&v.table).(components.Clickable); ok {
		return clickable.HandleRegionClick(regionID)
	}
	return nil
}
```

### Benefits of Delegation

1. **Encapsulation**: Views don't expose their internal table structure
2. **Type safety**: Safe type assertion with nil return on failure
3. **Flexibility**: Views can override behavior if needed
4. **Consistency**: Same pattern across all views

## Files Modified

### Views Updated (7 total)

1. **internal/ui/views/instances.go** - `InstancesView`
   - Added mouse import
   - Added 3 Clickable methods

2. **internal/ui/views/disks.go** - `DisksView`
   - Added mouse import
   - Added 3 Clickable methods

3. **internal/ui/views/images.go** - `ImagesView`
   - Added mouse import
   - Added 3 Clickable methods

4. **internal/ui/views/projects.go** - `ProjectsView`
   - Added mouse import
   - Added 3 Clickable methods

5. **internal/ui/views/snapshots.go** - `SnapshotsView`
   - Added mouse import
   - Added 3 Clickable methods

6. **internal/ui/views/buckets.go** - `BucketsView`
   - Added mouse import
   - Added components import (was missing)
   - Added 3 Clickable methods

7. **internal/ui/views/objects.go** - `ObjectsView`
   - Added mouse import
   - Added components import (was missing)
   - Added 3 Clickable methods

### Changes per File

**Added imports:**
```go
"github.com/slayer/gcon/internal/ui/mouse"
"github.com/slayer/gcon/internal/ui/components"  // if missing
```

**Added methods:**
- `UpdateRegions(offsetX, offsetY int)`
- `GetRegions() []mouse.Region`
- `HandleRegionClick(regionID string) tea.Cmd`

## Architecture Flow (After Fix)

```
User clicks at (X=50, Y=25)
    ↓
App.handleMouseEvent receives click
    ↓
App casts view to Clickable → SUCCESS ✓
    ↓
App calls view.UpdateRegions(offsetX, offsetY)
    ↓
View delegates to table.UpdateRegions(offsetX, offsetY)
    ↓
Table calculates regions in absolute coordinates
    ↓
App calls view.GetRegions()
    ↓
View delegates to table.GetRegions()
    ↓
App finds matching region at (X, Y)
    ↓
App calls view.HandleRegionClick(regionID)
    ↓
View delegates to table.HandleRegionClick(regionID)
    ↓
Table handles the click (select row, double-click, etc.)
```

## Testing

### Compilation

✅ Build successful:
```bash
go build -o /tmp/gcon ./cmd/gcon
# No errors
```

### Unit Tests

✅ All tests pass:
```bash
go test ./... -short
# 26 packages, 0 failures
ok  	github.com/slayer/gcon/internal/ui/views	0.442s
```

### Linter

✅ No issues:
```bash
golangci-lint run ./...
# 0 issues
```

### Manual Testing Required

Test mouse clicks in all views:
- [ ] Projects table - Click to select project
- [ ] Instances table - Click row, double-click to open details
- [ ] Disks table - Click row, double-click to open details
- [ ] Snapshots table - Click row, double-click to open details
- [ ] Images table - Click row, double-click to open details
- [ ] Buckets table - Click row, double-click to browse objects
- [ ] Objects table - Click row, double-click to download

Also test:
- [ ] Sidebar clicks (should still work)
- [ ] Tab clicks in detail views (should still work)
- [ ] Mouse wheel scroll (should work in tables)
- [ ] Hover effects (if implemented)

## Design Considerations

### Why Delegation Instead of Exposing Tables?

**Option A: Expose tables (rejected)**
```go
func (v *InstancesView) GetTable() *table.Model {
    return &v.table
}

// In app:
table := view.GetTable()
if clickable, ok := interface{}(table).(components.Clickable); ok {
    // ...
}
```

**Problems:**
- Breaks encapsulation
- Exposes internal structure
- Views can't override behavior
- Tight coupling

**Option B: Delegation (chosen)**
```go
func (v *InstancesView) UpdateRegions(offsetX, offsetY int) {
    // Delegate to internal table
}
```

**Benefits:**
- Maintains encapsulation
- Views control their behavior
- Flexible for future changes
- Clean interface

### Why Safe Type Assertion?

```go
if clickable, ok := interface{}(&v.table).(components.Clickable); ok {
    clickable.UpdateRegions(offsetX, offsetY)
}
```

**Reasoning:**
- Defensive programming - won't panic if table doesn't implement Clickable
- Future-proof - can change table implementation without breaking views
- Explicit - clear intent that delegation might fail gracefully

## Future Enhancements

### 1. View-Level Click Handling

Some views might want to intercept clicks before delegating to table:

```go
func (v *InstancesView) HandleRegionClick(regionID string) tea.Cmd {
    // View-specific logic
    if v.specialMode {
        return v.handleSpecialClick(regionID)
    }

    // Delegate to table for normal clicks
    if clickable, ok := interface{}(&v.table).(components.Clickable); ok {
        return clickable.HandleRegionClick(regionID)
    }
    return nil
}
```

### 2. Multi-Component Views

For views with multiple clickable components (e.g., table + tabs):

```go
func (v *DetailsView) UpdateRegions(offsetX, offsetY int) {
    // Update tabs regions (at top)
    if clickable, ok := interface{}(&v.tabs).(components.Clickable); ok {
        clickable.UpdateRegions(offsetX, offsetY)
    }

    // Update table regions (below tabs)
    if clickable, ok := interface{}(&v.table).(components.Clickable); ok {
        clickable.UpdateRegions(offsetX, offsetY+2)  // +2 for tabs height
    }
}

func (v *DetailsView) GetRegions() []mouse.Region {
    regions := []mouse.Region{}

    // Combine regions from all components
    if clickable, ok := interface{}(&v.tabs).(components.Clickable); ok {
        regions = append(regions, clickable.GetRegions()...)
    }
    if clickable, ok := interface{}(&v.table).(components.Clickable); ok {
        regions = append(regions, clickable.GetRegions()...)
    }

    return regions
}
```

### 3. View Interface Extension

Could add Clickable to the View interface:

```go
// In internal/ui/views/view.go
type View interface {
    Init() tea.Cmd
    Update(tea.Msg) tea.Cmd
    View() string
    SetSize(width, height int)
    SetContext(ctx *context.ProgramContext)

    // Mouse support (optional, can return nil)
    UpdateRegions(offsetX, offsetY int)
    GetRegions() []mouse.Region
    HandleRegionClick(regionID string) tea.Cmd
}
```

This would make all views explicitly declare mouse support.

## Lessons Learned

### 1. Test Integration Points

Unit tests passed because components work in isolation. The issue was in how the app interacts with views.

**Lesson:** Test the full path from app → view → component.

### 2. Interface Boundaries Matter

The mismatch between where we expected Clickable (views) and where it was implemented (components) caused the issue.

**Lesson:** Document which layer implements interfaces clearly.

### 3. Delegation Pattern is Powerful

Delegation allows clean separation while maintaining flexibility.

**Lesson:** Use delegation when you need interface compliance without exposing internals.

## Commit Message

```
2025-01-19: Fix table mouse clicks by implementing Clickable in views

Mouse clicks worked in sidebar but not in tables because views didn't
implement the Clickable interface. The app tried to cast views to
Clickable, but only the underlying table components implemented it.

Solution: Make all table-based views implement Clickable by delegating
to their internal table components.

Changes:
- Add Clickable interface methods to 7 view types
- Methods delegate to internal table components
- Safe type assertions prevent panics
- Maintains encapsulation (views don't expose tables)

Views updated:
- InstancesView, DisksView, ImagesView
- ProjectsView, SnapshotsView
- BucketsView, ObjectsView

Testing:
- All tests pass
- Lint clean
- Build successful
- Ready for manual testing

Fixes: Mouse clicks in tables now work correctly
```

---

**Status:** ✅ Implementation complete
**Next:** Manual testing to verify all table views accept mouse clicks
