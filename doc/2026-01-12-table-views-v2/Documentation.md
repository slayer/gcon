# Fix Terminal Rendering Glitches in Table Views

## Summary of Changes

Fixed terminal rendering glitches that occurred when:
1. Multiple pages of paginator (like in VM list with many instances)
2. VM detail page content is too long
3. Content height doesn't match sidebar height in `lipgloss.JoinHorizontal()`

## Root Cause

The issue was a **newline count mismatch** between sidebar and content when using `lipgloss.JoinHorizontal()`.

When sidebar uses `lipgloss.Height(h)`, it outputs `h-1` newlines. Content views were only padding to the correct height in loading states, but not in normal states. This caused the sidebar-content join to break, resulting in visual glitches.

## Solution

### 1. Created `padToHeight` Helper Function

Added `internal/ui/views/helpers.go` with a shared helper function:

```go
func padToHeight(content string, height int) string {
    style := lipgloss.NewStyle().Height(height)
    return style.Render(content)
}
```

**Critical insight**: This function uses `lipgloss.Height()` - the **exact same mechanism** the sidebar uses. This ensures identical output dimensions regardless of content length.

Manual string manipulation (counting/adding/truncating newlines) does NOT produce the same result as `lipgloss.Height()` when content overflows - lipgloss handles overflow by clipping content properly while maintaining the exact line count.

### 2. Modified All Views to Use Consistent Height

Updated the `View()` method in all views to:
1. Build content into a single variable
2. Call `padToHeight(content, v.height)` before returning

Modified files:
- `internal/ui/views/instances.go`
- `internal/ui/views/buckets.go`
- `internal/ui/views/objects.go`
- `internal/ui/views/instance_details.go`
- `internal/ui/views/projects.go`

### 3. Removed Redundant Code

Removed the per-view `renderLoading()` functions since height padding is now handled uniformly by `padToHeight()`.

## Technical Details

### Before (Inconsistent)

```go
func (v *InstancesView) View() string {
    if v.loading {
        return v.renderLoading("Loading...")  // Padded to height-1
    }
    return v.table.View() + help  // NOT padded - variable height!
}
```

### After (Consistent)

```go
func (v *InstancesView) View() string {
    var content string
    if v.loading {
        content = "Loading..."
    } else {
        content = v.table.View() + help
    }
    return padToHeight(content, v.height)  // Always padded
}
```

## Testing

Updated tests in:
- `internal/ui/views/instances_test.go` - verify View() output height consistency
- `internal/ui/views/buckets_test.go` - verify View() output height consistency

## Files Changed

```
internal/ui/views/helpers.go          # New file with padToHeight helper
internal/ui/views/instances.go        # Refactored View() method
internal/ui/views/buckets.go          # Refactored View() method
internal/ui/views/objects.go          # Refactored View() method
internal/ui/views/instance_details.go # Refactored View() method
internal/ui/views/projects.go         # Refactored View() method
internal/ui/views/instances_test.go   # Updated tests
internal/ui/views/buckets_test.go     # Updated tests
```

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                          App.View()                             │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │                    renderWithSidebar()                     │ │
│  │                                                           │ │
│  │  ┌─────────────┐   lipgloss.JoinHorizontal   ┌──────────┐│ │
│  │  │   Sidebar   │ ────────────────────────── │  Content ││ │
│  │  │             │                             │          ││ │
│  │  │ Height(h)   │                             │ padTo-   ││ │
│  │  │ = h-1       │       Must Match!           │ Height() ││ │
│  │  │ newlines    │ ◄─────────────────────────► │ = h-1    ││ │
│  │  │             │                             │ newlines ││ │
│  │  └─────────────┘                             └──────────┘│ │
│  └───────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```
