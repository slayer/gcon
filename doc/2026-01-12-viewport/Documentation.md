# Viewport-Wrapped Content Views - Documentation

## Summary of Changes

Wrapped content views (InstancesView, BucketsView, ObjectsView) with `bubbles/viewport` component to provide consistent height handling and automatic scrolling capability.

## Files Modified

- `internal/ui/views/instances.go` - Added viewport wrapper
- `internal/ui/views/buckets.go` - Added viewport wrapper
- `internal/ui/views/objects.go` - Added viewport wrapper with overlay support

## Technical Details

### Pattern Used

Each view now follows the viewport pattern from InstanceDetailsView:

```go
// Fields added to view struct
viewport viewport.Model
ready    bool // viewport initialized

// Lazy initialization in SetSize()
if !v.ready {
    v.viewport = viewport.New(width, viewportHeight)
    v.ready = true
} else {
    v.viewport.Width = width
    v.viewport.Height = viewportHeight
}

// Content wrapping in View()
if v.ready {
    v.viewport.SetContent(content)
    return v.viewport.View()
}

// Scroll handling in Update()
if v.ready {
    v.viewport, cmd = v.viewport.Update(msg)
}
```

### Height Calculation

The key insight is that `viewport.Height = N` causes `viewport.View()` to output `N-1` newlines. This matches how lipgloss `Height(N)` works for the sidebar, ensuring proper horizontal alignment when using `lipgloss.JoinHorizontal()`.

```
Sidebar: Height(36) -> outputs 35 newlines
Content: viewport.Height = 36 -> outputs 35 newlines
Result: Perfect alignment when joined horizontally
```

### ObjectsView Overlay Handling

For ObjectsView, overlays (file picker, progress bars, confirm dialogs) must render AFTER the viewport wrapping:

```go
// 1. Build content
content := v.list.View() + status + help

// 2. Wrap in viewport (ensures consistent height)
if v.ready {
    v.viewport.SetContent(content)
    content = v.viewport.View()
}

// 3. Apply overlays ON TOP of viewport output
if v.showFilePicker {
    content = v.overlayFilePicker(content)
}
// ... other overlays
```

### Minimum Height Fallback

For edge cases where height is very small (< 11), the code falls back to manual padding to ensure a minimum of 10 newlines output, matching the original behavior for test compatibility.

## Benefits

1. **Automatic scrolling**: Content that exceeds viewport height can be scrolled
2. **Consistent height**: No more manual newline counting for most cases
3. **Official component**: Uses well-tested Bubble Tea component
4. **Natural UX**: Users expect scrollable content in TUI applications

## Testing

All existing tests pass:
- `TestInstancesView_RenderingHeightConsistency`
- `TestInstancesView_RenderLoadingHeight`
- `TestApp_RenderingHeightConsistency`
- `TestApp_InstanceDetailsRenderingHeight`

Linter clean: `0 issues`

## Usage

The viewport is transparent to users - existing keyboard navigation (j/k, arrow keys) and mouse scrolling should work automatically for content that exceeds the viewport height.
