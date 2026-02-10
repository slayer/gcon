# Sidebar Auto-Hide Mode

## Summary

Added an auto-hide mode to the sidebar that behaves like IDE sidebars: collapsed by default, expands on interaction, auto-collapses after selecting a menu item. The `{` key toggles between auto-hide (default) and always-open (pinned) modes.

## Changes

### Sidebar Component (`internal/ui/components/sidebar/sidebar.go`)
- Added `SidebarMode` type with `SidebarModeAutoHide` and `SidebarModeAlwaysOpen` constants
- Added `mode` field to `Sidebar` struct
- `New()` now starts collapsed with `SidebarModeAutoHide`
- Added `Collapse()`, `Expand()`, `Mode()`, and `SetMode()` methods
- `SetMode(AutoHide)` collapses, `SetMode(AlwaysOpen)` expands

### App Key Handlers (`internal/ui/app.go`)
- `{` now toggles mode instead of toggling visibility
- `[` expands sidebar when focusing it in auto-hide mode
- `]` collapses sidebar when switching to content in auto-hide mode
- `Esc` at sidebar root unfocuses and collapses in auto-hide mode (previously would quit)

### Navigation (`internal/ui/app_navigation.go`)
- After leaf item selection, sidebar auto-collapses if in auto-hide mode
- Mouse click on collapsed sidebar in auto-hide mode: expand + focus (no item selection)

### Command Palette (`internal/ui/app_commands.go`)
- "Toggle sidebar" action now toggles mode (matches `{` key behavior)

### Tests (`internal/ui/components/sidebar/sidebar_test.go`)
- Updated `TestNew` to assert `collapsed: true` and auto-hide mode
- Updated `TestToggleCollapsed` for new initial state
- Updated `TestRenderHeader` to expand sidebar before checking text
- Added: `TestSidebarMode_DefaultAutoHide`, `TestSetMode_AutoHide_Collapses`, `TestSetMode_AlwaysOpen_Expands`, `TestCollapse_Idempotent`, `TestExpand_Idempotent`

## Behavior

| Action | Auto-Hide Mode | Always-Open Mode |
|--------|---------------|-----------------|
| App starts | Sidebar collapsed | Sidebar expanded |
| `[` | Expand + focus sidebar | Focus sidebar |
| `]` | Unfocus + collapse sidebar | Unfocus sidebar |
| `{` | Switch to always-open | Switch to auto-hide |
| Select leaf item | Navigate + collapse | Navigate (stays open) |
| Esc at sidebar root | Unfocus + collapse | Unfocus |
| Click collapsed strip | Expand + focus | N/A (already expanded) |

## Edge Cases
- Category drill-down keeps sidebar expanded (no NavigateMsg emitted)
- Mode survives `clearAllViews()` (user preference, not view state)
- `{` while focused in auto-hide: switches to always-open, sidebar stays expanded
