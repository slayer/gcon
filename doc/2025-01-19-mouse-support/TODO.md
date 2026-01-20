# Mouse Support Implementation - TODO

## Task Description
Implement comprehensive mouse support for gcon TUI application, allowing users to interact with the interface using mouse clicks, scroll wheels, and hover effects.

## Implementation Plan

### Phase 1: Foundation (Week 1) ✅ COMPLETED
Goal: Enable mouse and implement core navigation

- [x] 1.1 Enable mouse mode at startup
  - [x] Modify `cmd/gcon/main.go` to add `tea.WithMouseCellMotion()`
  - [x] Test mouse events are received
  - [x] Add `--no-mouse` CLI flag for accessibility

- [x] 1.2 Add mouse message routing in app.go
  - [x] Create mouse event dispatcher in `internal/ui/app_navigation.go`
  - [x] Pass mouse events to active view
  - [x] Handle sidebar width offset calculation

- [x] 1.3 Add hover styles
  - [x] Add hover style definitions to `internal/ui/styles.go`
  - [x] Define color scheme for hover states

- [x] 1.4 Implement Table component mouse support
  - [x] Add layout tracking (Y offset calculation)
  - [x] Implement coordinate-to-row mapping
  - [x] Add click-to-select functionality
  - [x] Add double-click-to-confirm functionality (500ms threshold)
  - [x] Add scroll wheel support
  - [x] Add hover state tracking
  - [x] All existing tests pass

- [x] 1.5 Implement Sidebar component mouse support
  - [x] Add layout tracking (Y offset calculation)
  - [x] Implement coordinate-to-item mapping
  - [x] Add click-to-select functionality
  - [x] Add click-to-drill functionality
  - [x] Add hover state tracking
  - [x] Handle collapsed state (coordinates work in both modes)
  - [x] All existing tests pass

- [x] 1.6 Implement Tabs component mouse support
  - [x] Add layout tracking (X coordinate calculation)
  - [x] Implement coordinate-to-tab mapping
  - [x] Add click-to-switch functionality
  - [x] Add hover state tracking
  - [x] Test in Instance and Disk detail views (code level)

- [x] 1.7 Phase 1 testing and refinement
  - [x] Build verification (all components compile)
  - [x] Unit tests (all 24 packages passing)
  - [x] Linter verification (0 issues)
  - [x] Code formatting with `go fmt`
  - [x] Staticcheck warnings resolved

### Phase 2: Interactive Elements (Week 2)
Goal: Add mouse support to overlays and dialogs

- [ ] 2.1 Implement Action Menu mouse support
  - [ ] Add layout tracking with overlay positioning
  - [ ] Add coordinate-to-item mapping
  - [ ] Add click-to-select functionality
  - [ ] Add click-outside-to-close functionality
  - [ ] Add hover highlighting
  - [ ] Test across all contexts

- [ ] 2.3 Implement Confirmation Dialog mouse support
  - [ ] Add button coordinate mapping
  - [ ] Add click Yes/No functionality
  - [ ] Add click-outside-to-cancel functionality
  - [ ] Test delete confirmations

- [ ] 2.4 Phase 2 testing
  - [ ] Manual testing checklist
  - [ ] Integration tests
  - [ ] User feedback collection

### Phase 3: Polish (Week 3)
Goal: Command palette, file picker, and refinements

- [ ] 3.1 Implement Command Palette mouse support
  - [ ] Add filtered list coordinate mapping
  - [ ] Add click-to-select functionality
  - [ ] Add scroll wheel support
  - [ ] Test fuzzy search interaction

- [ ] 3.2 Implement File Picker mouse support
  - [ ] Add file list coordinate mapping
  - [ ] Add click-to-select functionality
  - [ ] Test upload flow

- [ ] 3.3 Add hover effects throughout
  - [ ] Review all components for consistency
  - [ ] Add subtle hover animations if appropriate
  - [ ] Visual polish pass

- [ ] 3.4 Performance testing
  - [ ] Monitor event handling performance
  - [ ] Optimize if necessary
  - [ ] Test with motion events if needed

- [ ] 3.5 Documentation
  - [ ] Update README with mouse support info
  - [ ] Add `--no-mouse` CLI flag documentation
  - [ ] Update user guide
  - [ ] Create mermaid diagrams for mouse event flow

- [ ] 3.6 Final testing and bug fixes
  - [ ] Comprehensive manual testing
  - [ ] Terminal compatibility testing
  - [ ] Bug triage and fixes

## Technical Requirements

### Coordinate Translation System
- Track component layouts during render phase
- Account for sidebar offset in content area
- Handle scrolling offsets in scrollable components
- Implement bounds checking for all click areas

### Visual Feedback
- Maintain distinction between cursor (keyboard) and hover (mouse)
- Render priority: selected > hover > default
- Consistent hover style across all components

### Accessibility
- All mouse actions must have keyboard equivalents
- Keyboard-first design preserved
- Add `--no-mouse` CLI flag for accessibility

## Testing Strategy

### Manual Testing
- Click table rows across all views
- Double-click to confirm
- Sidebar menu navigation
- Tab switching
- Action menu item selection
- Confirmation dialog buttons
- Click-outside-to-close overlays
- Scroll wheel in tables/sidebar
- Hover effects appear correctly
- Keyboard shortcuts still work
- Mouse + keyboard combined usage

### Automated Testing
- Unit tests for coordinate mapping functions
- Mock mouse events for component tests
- Integration tests for app-level routing

## Files to Modify

| File | Change Type | Status |
|------|-------------|--------|
| `cmd/gcon/main.go` | Add mouse option | ⏳ Pending |
| `internal/ui/app.go` | Route mouse messages | ⏳ Pending |
| `internal/ui/styles.go` | Add hover styles | ⏳ Pending |
| `internal/ui/components/table/table.go` | Click handling | ⏳ Pending |
| `internal/ui/components/sidebar/sidebar.go` | Click handling | ⏳ Pending |
| `internal/ui/components/tabs/tabs.go` | Click handling | ⏳ Pending |
| `internal/ui/components/actionmenu/actionmenu.go` | Click handling | ⏳ Pending |
| `internal/ui/components/confirm/confirm.go` | Click handling | ⏳ Pending |
| `internal/ui/components/commandpalette/commandpalette.go` | Click handling | ⏳ Pending |
| `internal/ui/components/filepicker/filepicker.go` | Click handling | ⏳ Pending |

## Risks and Mitigations

- **Terminal Compatibility**: Always maintain keyboard alternatives
- **Coordinate Calculation Bugs**: Comprehensive unit tests, visual debugging
- **Performance**: Use `WithMouseCellMotion()`, debounce if needed
- **Layout Changes**: Centralize layout tracking, add bounds tests

## Success Criteria

- All table rows clickable across all views
- Sidebar navigation fully functional with mouse
- All overlays support click-to-close
- Hover effects provide clear visual feedback
- No regression in keyboard functionality
- Works on major terminal emulators (iTerm2, Terminal.app, Alacritty, etc.)
