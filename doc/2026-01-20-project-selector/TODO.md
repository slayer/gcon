# TODO: Project Selection Modal Dialog

## Overview
Implement a modal project selection dialog for quick project switching without leaving the current view.

## Implementation Tasks

### Phase 1: Create ProjectSelector Component
- [x] Create `internal/ui/components/projectselector/messages.go`
  - Define ProjectSelectedMsg
  - Define ProjectSelectorCanceledMsg
  - Define internal messages (projectsLoadedMsg, projectsErrorMsg)
- [x] Create `internal/ui/components/projectselector/projectselector.go`
  - Implement Model struct with filtering support
  - Implement New() constructor
  - Implement Init() for async project loading
  - Implement Update() with keyboard navigation and filtering
  - Implement View() for centered modal overlay
  - Add empty states and error handling

### Phase 2: Integrate into App
- [x] Modify `internal/ui/app.go`
  - Add projectSelector and showProjectSelector fields to App struct
  - Initialize projectSelector in NewApp()
  - Add ShowProjectSelectorOnStartup() method
  - Update Init() to handle project selector on startup
- [x] Modify `internal/ui/app_commands.go`
  - Add "switch-project" action command
  - Add handler in handleActionCommand()
- [x] Update `internal/ui/components/commandpalette/commands.go`
  - Register switch project command in available commands

### Phase 3: Handle Project Switching
- [x] Modify `internal/ui/app_navigation.go`
  - Implement handleProjectSwitch()
  - Implement clearAllViews()
  - Implement reloadCurrentView()
  - Add message handlers in main Update()

### Phase 4: Footer Click Integration
- [x] Modify `internal/ui/components/footer.go`
  - Add mouse event handling for Right3 zone
  - Define FooterProjectClickedMsg
  - Track project text zone for click detection
- [x] Modify `internal/ui/app_footer.go`
  - Handle FooterProjectClickedMsg to show selector
  - Update footer mouse zone tracking

**Note**: Footer click integration is implemented as an optional enhancement. Core functionality also works via the command palette.

### Phase 5: Rendering
- [x] Modify `internal/ui/app.go` View() method
  - Add project selector overlay rendering
  - Route messages to project selector when shown
- [x] Update main Update() method
  - Route input to project selector when active
  - Handle modal close events

### Phase 6: Startup Integration
- [x] Modify `cmd/gcon/main.go`
  - Detect when no default project is set
  - Show project selector on startup if needed
  - Handle initial project selection flow

### Phase 7: Testing
- [x] Create unit tests for projectselector component
- [x] Add integration tests for project switching
- [x] Manual testing checklist (see plan)

### Phase 8: Documentation
- [ ] Update README.md with new features
- [ ] Update key bindings documentation
- [ ] Create Documentation.md summarizing changes

## Notes
- Reuse ActionMenu modal styling pattern (fixed width: 80 characters, centered)
- Similar UX to command palette filtering
- Ensure view state cleanup on project switch
- Handle permission errors gracefully

## Bug Fixes

### Issue: Stuck on "Loading projects..."
**Problem**: Modal showed permanent loading spinner because internal messages weren't being routed to the project selector component.

**Root Cause**: `App.Update()` was only passing `tea.KeyMsg` to project selector, but not `spinner.TickMsg` or the internal `projectsLoadedMsg`/`projectsErrorMsg`.

**Solution**: Changed App.Update() to use a `default:` case that passes ALL messages to project selector when modal is active.

```go
if a.showProjectSelector {
    switch msg := msg.(type) {
    case projectselector.ProjectSelectedMsg:
        return a, a.handleProjectSwitch(&msg.Project)
    case projectselector.ProjectSelectorCanceledMsg:
        a.showProjectSelector = false
        return a, nil
    default:
        // Pass all other messages to project selector
        cmd := a.projectSelector.Update(msg)
        return a, cmd
    }
}
```

**Status**: ✅ Fixed in commit

### Issue: Help text background highlight
**Problem**: Help text at bottom of modal (`j/k:nav  enter:select  esc:cancel`) showed gray background.

**Root Cause**: Lipgloss styles (even with `UnsetBackground()`) were being wrapped by the Container style which added padding/background. When rendering with `.Width()`, lipgloss fills remaining space with background color.

**Solution**: Render help text as plain string without any lipgloss styling, added directly to string builder before container renders.

**Status**: ✅ Fixed - help text now displays without background inside modal border
