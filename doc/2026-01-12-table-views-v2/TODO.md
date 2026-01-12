# Fix Terminal Rendering Glitches in Table Views

## Problem Description

Terminal rendering glitches occur when:
1. Multiple pages of paginator (like in VM list)
2. VM show page is too long
3. Content height doesn't match sidebar height in `lipgloss.JoinHorizontal()`

## Root Cause Analysis

The issue is a **newline count mismatch** between sidebar and content when using `lipgloss.JoinHorizontal()`.

### Key Insight from CLAUDE.md

> `lipgloss.Height(n)` renders n lines, which equals **n-1 newlines**.

The sidebar uses:
```go
container := styles.Container.Width(width).Height(s.height)
return container.Render(content)  // outputs height-1 newlines
```

Content views try to match but have inconsistent output:

1. **Loading states** - have `renderLoading()` that pads to `height-1` newlines (correct)
2. **Normal states** - don't pad at all, causing variable height based on content

### Affected Views

| View | Loading State | Normal State | Issue |
|------|--------------|--------------|-------|
| `instances.go` | Pads to height-1 | No padding | Normal view height varies |
| `buckets.go` | Pads to height-1 | No padding | Normal view height varies |
| `objects.go` | Pads to height-1 | No padding | Normal view + overlays break |
| `instance_details.go` | Pads to height-1 | No padding | Viewport + help varies |
| `projects.go` | No padding | No padding | Less critical (no sidebar) |

## Implementation Plan

### Step 1: Create Helper Function

Add a `padToHeight` helper in a shared location that all views can use:

```go
// padToHeight ensures content has exactly the specified number of newlines
// for consistent rendering when using lipgloss.JoinHorizontal().
//
// When sidebar uses lipgloss.Height(h), it outputs h-1 newlines.
// Content must match with exactly h-1 newlines.
func padToHeight(content string, height int) string {
    targetNewlines := height - 1
    if targetNewlines < 1 {
        targetNewlines = 1
    }
    currentNewlines := strings.Count(content, "\n")

    if currentNewlines < targetNewlines {
        content += strings.Repeat("\n", targetNewlines-currentNewlines)
    } else if currentNewlines > targetNewlines {
        // Truncate excess newlines by removing trailing ones
        lines := strings.Split(content, "\n")
        if len(lines) > targetNewlines+1 {
            lines = lines[:targetNewlines+1]
            content = strings.Join(lines, "\n")
        }
    }
    return content
}
```

### Step 2: Fix Each View's View() Method

For each view that renders with sidebar:

1. Calculate final content string
2. Call `padToHeight(content, v.height)` before returning
3. Remove redundant padding in `renderLoading()` (use shared helper)

### Step 3: Test Height Consistency

Add test to verify sidebar and content have matching newline counts.

## Tasks

- [x] Analyze codebase and identify root cause
- [x] Create `padToHeight` helper function
- [x] Fix `instances.go` View() method
- [x] Fix `buckets.go` View() method
- [x] Fix `objects.go` View() method
- [x] Fix `instance_details.go` View() method
- [x] Fix `projects.go` View() method (for consistency)
- [x] Run tests and verify fix
- [x] Run linter
