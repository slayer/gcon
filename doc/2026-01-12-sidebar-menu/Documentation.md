# Sidebar Menu - Technical Documentation

## Overview

This feature adds a collapsible sidebar menu to gcon that appears after project selection. The sidebar provides GCP Console-like navigation between different resource types (Compute Engine, Cloud Storage, VPC Network).

## Architecture

### Component Structure

```
internal/ui/components/sidebar/
├── menu.go      # Data structures and menu hierarchy
├── styles.go    # Lip Gloss styles for sidebar
├── sidebar.go   # Main component logic
└── sidebar_test.go
```

### Navigation Flow

```
┌─────────────────────────────────────────────────────────────┐
│ Projects View (Full Width - No Sidebar)                     │
└────────────────────────┬────────────────────────────────────┘
                         │ Project Selected
                         v
┌─────────┬───────────────────────────────────────────────────┐
│ Sidebar │ Content View (VM Instances / Disks / etc.)        │
│         │                                                   │
│ Compute │ Tab switches focus between panels                 │
│ Storage │ [ toggles sidebar width                           │
│ Network │                                                   │
└─────────┴───────────────────────────────────────────────────┘
```

### Drill-down Navigation

The sidebar uses drill-down navigation instead of tree expansion:

```
Root Level:              Drilled into Compute:
┌──────────────────┐    ┌──────────────────┐
│  Compute Engine ›│    │ < Back           │
│  Cloud Storage  ›│    │                  │
│  VPC Network    ›│    │   VM instances   │
└──────────────────┘    │   Disks          │
                        └──────────────────┘
```

## Key Data Structures

### MenuItem

```go
type MenuItem struct {
    ID       string       // Unique identifier
    Label    string       // Display text
    Icon     string       // Icon for collapsed mode
    Type     MenuItemType // Category or Leaf
    ViewType ViewType     // Target view (for leaves)
    Children []MenuItem   // Child items (for categories)
}
```

### Sidebar State

```go
type Sidebar struct {
    menu         []MenuItem // Root menu
    currentItems []MenuItem // Currently displayed (changes on drill-down)
    path         []string   // Breadcrumb trail
    cursor       int        // Selected index
    collapsed    bool       // Icon-only mode
    focused      bool       // Has keyboard focus
    activeView   ViewType   // Highlighted item
}
```

## Focus Management

The app tracks which panel has keyboard focus:

```go
type FocusedPanel int
const (
    FocusContent FocusedPanel = iota
    FocusSidebar
)
```

- `Tab` / `Shift+Tab`: Toggle focus between sidebar and content
- When sidebar is focused: j/k navigate, Enter selects
- When content is focused: Normal view key bindings apply

## Message Flow

```
User presses Enter on "VM instances"
         │
         v
Sidebar emits NavigateMsg{ViewType: ViewInstances, ItemID: "vm-instances"}
         │
         v
App.Update() catches sidebar.NavigateMsg
         │
         v
handleSidebarNavigation() switches currentView and updates sidebar highlight
```

## Key Bindings

| Key | Scope | Action |
|-----|-------|--------|
| `Tab` | Global | Toggle focus |
| `[` | Global | Toggle sidebar collapsed |
| `j/k` | Sidebar | Move cursor |
| `Enter/l/→` | Sidebar | Select item |
| `h/←/Backspace` | Sidebar | Go back |
| `Esc` | Sidebar | Go back (if drilled down) |

## Styling

The sidebar uses GCP-inspired colors:
- Primary (Google Blue): `#4285F4` - selected items, focused border
- Muted (Gray): `#9AA0A6` - normal text, borders

Collapsed width: 6 chars (icon + padding)
Expanded width: 24 chars

## Testing

Run sidebar tests:
```bash
go test ./internal/ui/components/sidebar/...
```

Tests cover:
- Navigation (up/down/select/back)
- Drill-down into categories
- Leaf item selection (NavigateMsg emission)
- Collapsed/expanded toggle
- Focus state handling

## Future Enhancements

1. **Implement actual views** for Disks, Buckets, Networks, Firewall
2. **Add more resources**: IAM, GKE, Cloud Run, Cloud SQL
3. **Search in sidebar**: Quick filter menu items
4. **Persist collapsed state** to config file
5. **Keyboard shortcut hints** in sidebar items
