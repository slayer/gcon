# Sidebar Auto-Hide Mode

## Task
Implement auto-hide sidebar behavior: hidden by default, auto-collapses after leaf item selection. `{` toggles between auto-hide and always-open (pinned) modes.

## Implementation Steps

- [x] Add SidebarMode type and methods to sidebar component
- [x] Update `New()` to start collapsed in auto-hide mode
- [x] Update `{` key handler to toggle mode instead of toggle visibility
- [x] Update `[` key handler to expand sidebar in auto-hide mode
- [x] Update `]` key handler to collapse sidebar in auto-hide mode
- [x] Handle Esc when sidebar focused at root (unfocus + collapse in auto-hide)
- [x] Auto-collapse after leaf selection in handleSidebarNavigation
- [x] Handle mouse click on collapsed sidebar (expand + focus in auto-hide)
- [x] Update command palette toggle-sidebar action
- [x] Update key help text for `{`
- [x] Update sidebar tests
- [x] Update documentation (key-bindings.md, README.md, navigation.md)
- [x] Run tests and linter
