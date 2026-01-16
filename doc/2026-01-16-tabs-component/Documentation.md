# Tabs and Links Components Documentation

## Overview

Two reusable components for detail views in gcon:

1. **Tabs Component** - Tab bar for switching between content sections
2. **Links Component** - Navigable links within content (e.g., disk links that navigate to disk details)

---

## Tabs Component

### Creating Tabs

```go
import "github.com/slayer/gcon/internal/ui/components/tabs"

tabsComponent := tabs.New([]tabs.Tab{
    {ID: "details", Label: "Details"},
    {ID: "observability", Label: "Observability"},
})
```

### Key Methods

| Method | Description |
|--------|-------------|
| `New(tabs []Tab) *Tabs` | Create new tabs component |
| `Update(msg tea.Msg) tea.Cmd` | Handle key events, returns `TabChangedMsg` on tab change |
| `View() string` | Render the tab bar |
| `ActiveTab() Tab` | Get currently active tab |
| `ActiveIndex() int` | Get index of active tab |
| `SetActive(idx int)` | Set active tab by index |
| `HandleKey(msg tea.KeyMsg) bool` | Check if key should be handled by tabs |

### Navigation Keys

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Next/Previous tab |
| `h` / `l` or `←` / `→` | Previous/Next tab |
| `1-9` | Jump to tab N |

### Visual Style

```
 [Details]  Observability
```
- Active tab: `[Label]` with bold GCP blue (#4285F4)
- Inactive tab: ` Label ` with muted gray (#9AA0A6)

---

## Links Component

### Creating Links

```go
import "github.com/slayer/gcon/internal/ui/components/links"

linksComponent := links.New()
linksComponent.SetItems([]links.Link{
    {ID: "disk1", Label: "boot-disk", Type: "disk", Data: diskInfo},
    {ID: "disk2", Label: "data-disk", Type: "disk", Data: diskInfo2},
})
```

### Key Methods

| Method | Description |
|--------|-------------|
| `New() *Links` | Create new links component |
| `SetItems(items []Link)` | Set the list of navigable links |
| `Update(msg tea.Msg) tea.Cmd` | Handle key events, returns `LinkSelectedMsg` on Enter |
| `RenderRow(index int, row string) string` | Render a row with focus highlighting |
| `RenderHeader(header string) string` | Render a table header |
| `RenderDivider(width int) string` | Render a divider line |
| `FocusedIndex() int` | Get currently focused link index |
| `HasItems() bool` | Check if there are navigable links |
| `HandleKey(msg tea.KeyMsg) bool` | Check if key should be handled by links |

### Navigation Keys

| Key | Action |
|-----|--------|
| `j` / `k` or `↓` / `↑` | Move between links |
| `Enter` | Select/navigate to link |

### Visual Style

```
  Name                      Size       Type         Mode         Boot
  ──────────────────────────────────────────────────────────────────────
▶ my-boot-disk              100 GB     pd-ssd       READ_WRITE   Yes
  my-data-disk              500 GB     pd-standard  READ_WRITE   —
```
- Focused row: Cursor (▶) + blue background highlighting (#4285F4)
- Normal row: No cursor, white text

---

## Integration Pattern

### Example: Instance Details View

```go
type InstanceDetailsView struct {
    tabs         *tabs.Tabs
    tabViewports []viewport.Model
    diskLinks    *links.Links
    // ...
}

func (v *InstanceDetailsView) Update(msg tea.Msg) tea.Cmd {
    switch msg := msg.(type) {
    case tabs.TabChangedMsg:
        v.updateViewportContent()
        return nil

    case links.LinkSelectedMsg:
        // Handle navigation to linked resource
        if msg.Link.Type == "disk" {
            return func() tea.Msg {
                return InstanceDiskSelectedMsg{...}
            }
        }
        return nil

    case tea.KeyMsg:
        // Route tab keys
        if tabs.HandleKey(msg) {
            return v.tabs.Update(msg)
        }

        // Route link keys in Details tab
        if v.tabs.ActiveTab().ID == "details" && v.diskLinks.HasItems() {
            if links.HandleKey(msg) {
                cmd := v.diskLinks.Update(msg)
                v.updateViewportContent()
                return cmd
            }
        }
    }
    return nil
}

func (v *InstanceDetailsView) renderDetailsTab() string {
    var b strings.Builder
    // ... other content ...

    // Render disk table with links
    if len(disks) > 0 {
        b.WriteString(v.diskLinks.RenderHeader("Name  Size  Type"))
        b.WriteString("\n")
        b.WriteString(v.diskLinks.RenderDivider(60))
        b.WriteString("\n")
        for i, disk := range disks {
            row := fmt.Sprintf("%-20s %-10s %-12s", disk.Name, disk.Size, disk.Type)
            b.WriteString(v.diskLinks.RenderRow(i, row))
            b.WriteString("\n")
        }
    }
    return b.String()
}
```

---

## Message Types

### Tabs

```go
type TabChangedMsg struct {
    TabID string  // ID of the newly active tab
    Index int     // Index of the newly active tab
}
```

### Links

```go
type LinkSelectedMsg struct {
    Link Link     // The selected link with ID, Label, Type, and Data
}
```

---

## Testing

Both components have comprehensive tests:
- `tabs_test.go` - Tab navigation, number keys, view rendering
- `links_test.go` - Link navigation, selection, rendering, edge cases
