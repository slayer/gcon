# Sidebar Menu Implementation

## Task Description

Add a collapsible sidebar menu (GCP Console-like) that appears after project selection, allowing navigation between resource types with drill-down submenus.

## Requirements

- Sidebar always visible after project selection (left side)
- Collapsible: toggle between full text and icon-only mode
- Drill-down navigation: selecting category replaces menu with children + back button
- Initial resources: Compute Engine (VM, Disks), Cloud Storage (Buckets), VPC Network (Networks, Firewall)

## Implementation Plan

### Phase 1: Sidebar Component Foundation
- [x] Create `internal/ui/components/sidebar/` directory
- [x] Implement `menu.go` - MenuItem struct, DefaultMenu()
- [x] Implement `styles.go` - sidebar-specific styles
- [x] Implement `sidebar.go` - basic rendering
- [x] Add tests for menu navigation

### Phase 2: App Integration
- [x] Add sidebar field to App struct
- [x] Modify `App.View()` for two-panel layout
- [x] Add focus management (Tab/Shift+Tab)
- [x] Handle `sidebar.NavigateMsg` in app.go
- [x] Update keys.go with `ToggleSidebar` binding

### Phase 3: Drill-down Navigation
- [x] Implement `DrillDown()` method
- [x] Implement `GoBack()` method
- [x] Render "< Back" item
- [x] Connect leaf selection to view switching
- [x] Highlight active view in sidebar

### Phase 4: Collapsed Mode
- [x] Add `Toggle()` method
- [x] Render icon-only view
- [x] Ensure navigation works in collapsed mode

### Phase 5: Placeholder Views
- [x] Create inline placeholders for Disks, Buckets, Networks, Firewall
- [x] Wire up to sidebar navigation

## Key Bindings

| Key | Context | Action |
|-----|---------|--------|
| `Tab` | Global | Focus: Sidebar → Content |
| `Shift+Tab` | Global | Focus: Content → Sidebar |
| `[` | Global | Toggle sidebar |
| `j/k`, `↓/↑` | Sidebar | Navigate items |
| `Enter`, `l`, `→` | Sidebar | Select/drill-down |
| `h`, `←`, `Backspace` | Sidebar | Go back |

## Files Created/Modified

**New files:**
- `internal/ui/components/sidebar/menu.go` - Menu data structures and GCP hierarchy
- `internal/ui/components/sidebar/styles.go` - Sidebar-specific styling
- `internal/ui/components/sidebar/sidebar.go` - Main component with navigation logic
- `internal/ui/components/sidebar/sidebar_test.go` - Unit tests

**Modified files:**
- `internal/ui/app.go` - Two-panel layout, focus management, sidebar integration
- `internal/ui/messages.go` - Added `SidebarNavigateMsg`
- `internal/ui/keys.go` - Added `ToggleSidebar` key binding

## Status: COMPLETE
