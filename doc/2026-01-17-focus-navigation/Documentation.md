# Universal Focus & Navigation System

## Summary of Changes

This PR introduces a focus management system that resolves key conflicts between multiple interactive components in detail views. The system allows users to cycle between focusable regions (tabs, links, viewport) using the Tab key, with context-sensitive key routing and help text.

## Problem Solved

In the instance details view with attached disks, `j/k` keys were captured by the links component, preventing viewport scrolling. Users had no way to scroll through the content when disk links were present.

## Solution Architecture

### Focus Package (`internal/ui/focus/`)

A new package that provides a unified focus management system:

```
internal/ui/focus/
├── region.go      # RegionType enum and Region struct
├── manager.go     # FocusManager implementation
├── messages.go    # FocusChangedMsg for cross-component communication
├── help.go        # Context-sensitive help text generation
└── *_test.go      # Comprehensive test coverage
```

### Region Types

The system supports these focus region types:

| Type | Key Behavior | Use Case |
|------|-------------|----------|
| `RegionViewport` | j/k scrolls | Content areas |
| `RegionLinks` | j/k navigates, Enter activates | Clickable link lists |
| `RegionTabs` | h/l switches, 1-9 direct | Tab bars |
| `RegionList` | j/k navigates, Enter selects | General lists |
| `RegionForm` | j/k navigates fields | Form inputs (future) |
| `RegionButtons` | h/l navigates, Enter presses | Button groups (future) |

### Focus Cycling

```
┌──────────────────────────────────────────────────────┐
│  Instance Details View                               │
├──────────────────────────────────────────────────────┤
│  [Tab Region] Details | Observability                │
│  ─────────────────────────────────────────────────── │
│  [Viewport Region]                                   │
│  Instance: my-vm  ● RUNNING                          │
│  ───────────────────────────────────────────────     │
│                                                      │
│  Storage                                             │
│  [Links Region]                                      │
│  → disk-1    100 GB   pd-balanced                    │
│    disk-2     50 GB   pd-ssd                         │
│                                                      │
├──────────────────────────────────────────────────────┤
│  Tab cycles: Tabs → Links → Viewport → Tabs...       │
└──────────────────────────────────────────────────────┘
```

## Key Mapping by Focus

| Focused Region | j/k | h/l | Tab | Enter |
|----------------|-----|-----|-----|-------|
| Tabs | - | Switch tab | Next region | - |
| Links | Select disk | - | Next region | Open disk |
| Viewport | Scroll | - | Next region | - |

## Usage Example

```go
// Initialize focus manager with regions
fm := focus.NewManager()
fm.SetRegions([]focus.Region{
    focus.NewRegion("tabs", focus.RegionTabs, "Tabs"),
    focus.NewDisabledRegion("links", focus.RegionLinks, "Disks"),  // Starts disabled
    focus.NewRegion("viewport", focus.RegionViewport, "Content"),
})

// In Update():
if focusMsg := fm.HandleKey(msg); focusMsg != nil {
    // Focus changed - update rendering
    return func() tea.Msg { return focusMsg }
}

// Route keys based on focused region
switch fm.ActiveType() {
case focus.RegionTabs:
    // Handle tab navigation
case focus.RegionLinks:
    // Handle link navigation
case focus.RegionViewport:
    // Handle viewport scrolling
}

// Enable/disable regions dynamically
fm.EnableRegion("links")   // When disks are loaded
fm.DisableRegion("links")  // When switching to Observability tab
```

## Context-Sensitive Help

The help text automatically updates based on the focused region:

- **Tabs focused**: `h/l: switch tab • 1-9: go to tab • tab: next region • .: actions`
- **Links focused**: `j/k: select disk • enter: open • tab: next region • .: actions`
- **Viewport focused**: `j/k: scroll • tab: next region • .: actions`

## Files Modified

| File | Changes |
|------|---------|
| `internal/ui/focus/region.go` | New - RegionType and Region definitions |
| `internal/ui/focus/manager.go` | New - FocusManager implementation |
| `internal/ui/focus/messages.go` | New - FocusChangedMsg |
| `internal/ui/focus/help.go` | New - Help text generation |
| `internal/ui/views/instance_details.go` | Integrated FocusManager |

## Testing

Run focus package tests:
```bash
go test -v ./internal/ui/focus/...
```

Run full test suite:
```bash
make test
```

## Manual Testing Checklist

1. Navigate to an instance with attached disks
2. Verify initial focus is on Tabs region (h/l switches tabs)
3. Press Tab - focus moves to Links region (j/k selects disks)
4. Press Tab - focus moves to Viewport region (j/k scrolls)
5. Press Tab - focus cycles back to Tabs region
6. Verify help text updates for each focused region
7. Switch to Observability tab - Links region is disabled
8. Tab only cycles between Tabs and Viewport

## Future Extensions

This design supports:
- **Forms**: Integrate with `charmbracelet/huh` for complex forms
- **Multi-view focus**: Coordinate multiple FocusManagers at app level
- **Accessibility**: Visual focus indicators, screen reader support
- **Other detail views**: Disk details, snapshot details, etc.
