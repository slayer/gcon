# Tabs and Links Component Implementation

## Task Description
1. Create a reusable tabs component for detail views, with initial integration in Instance Details view.
2. Create a reusable links component for navigable items within detail views, with initial integration for disk links in Instance Details.

## Status: Completed

## Implementation Summary

### Tabs Component

**Tab Bar Style:**
```
 [Details]  Observability
```
Active tab shown with brackets and GCP blue color, inactive tabs plain text in muted gray.

**Navigation Keys:**
| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Cycle tabs forward/backward |
| `h` / `l` or `←` / `→` | Move left/right between tabs |
| `1-9` | Jump directly to tab N |

### Links Component

**Highlighting Style:**
```
  Name                      Size       Type         Mode         Boot
  ──────────────────────────────────────────────────────────────────────
▶ my-boot-disk              100 GB     pd-ssd       READ_WRITE   Yes    ← focused row (highlighted)
  my-data-disk              500 GB     pd-standard  READ_WRITE   —
```
Focused link has cursor indicator (▶) and blue background highlighting.

**Navigation Keys:**
| Key | Action |
|-----|--------|
| `j` / `k` or `↓` / `↑` | Move between links |
| `Enter` | Navigate to linked resource |

## Completed Tasks

### Tabs Component
- [x] Create tabs component (`internal/ui/components/tabs/tabs.go`)
- [x] Create tabs styles (`internal/ui/components/tabs/styles.go`)
- [x] Create tabs tests (`internal/ui/components/tabs/tabs_test.go`)
- [x] Integrate tabs with Instance Details view
- [x] Add Details tab (existing content)
- [x] Add Observability tab (placeholder for future metrics)
- [x] Per-tab viewport scrolling (scroll position preserved when switching tabs)

### Links Component
- [x] Create links component (`internal/ui/components/links/links.go`)
- [x] Create links styles (`internal/ui/components/links/styles.go`)
- [x] Create links tests (`internal/ui/components/links/links_test.go`)
- [x] Integrate links with Instance Details view for disk navigation
- [x] Add `InstanceDiskSelectedMsg` for cross-view navigation
- [x] Update App to handle `InstanceDiskSelectedMsg` and navigate to Disk Details
- [x] Context-sensitive help text based on active tab and available links

## Files Created

- `internal/ui/components/tabs/tabs.go` - Main tabs component
- `internal/ui/components/tabs/styles.go` - Lipgloss styles for tabs
- `internal/ui/components/tabs/tabs_test.go` - Unit tests for tabs
- `internal/ui/components/links/links.go` - Main links component
- `internal/ui/components/links/styles.go` - Lipgloss styles for links
- `internal/ui/components/links/links_test.go` - Unit tests for links

## Files Modified

- `internal/ui/views/instance_details.go` - Integrated tabs and links components
- `internal/ui/app.go` - Added handler for `InstanceDiskSelectedMsg`

## Key Design Decisions

1. **Tabs**: Bracket style for active tab (`[Details]`) provides clear visual indication
2. **Links**: Table-style highlighting with cursor and background color (matching table component)
3. **Per-tab viewports**: Each tab has its own viewport to preserve scroll position
4. **HandleKey helpers**: Both components provide `HandleKey()` for parent routing decisions
5. **Message-based navigation**: `InstanceDiskSelectedMsg` allows App to coordinate view transitions
6. **Context-sensitive help**: Help text changes based on active tab and available links

## Future Enhancements

- Add Observability tab content (CPU, memory, network metrics)
- Add more linkable resources (networks, images, service accounts)
- Apply tabs to other detail views (Disk Details, Image Details)
- Apply links to other detail views (e.g., disk → source image)
