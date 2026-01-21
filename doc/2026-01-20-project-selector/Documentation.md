# Project Selector Modal Dialog - Documentation

## Overview

Implemented a modal project selection dialog that allows users to quickly switch between GCP projects without leaving their current view. The modal can be triggered from the command palette or shown automatically on startup when no default project is configured.

## Implementation Summary

### New Components

#### 1. Project Selector Component (`internal/ui/components/projectselector/`)

- **`messages.go`**: Message types for project selection events
  - `ProjectSelectedMsg`: Emitted when user selects a project
  - `ProjectSelectorCanceledMsg`: Emitted when user cancels the modal
  - Internal messages for async project loading

- **`projectselector.go`**: Main modal component
  - Async project loading from GCP API
  - Real-time text filtering by project name or ID
  - Keyboard navigation (↑/↓, j/k, Ctrl-N/Ctrl-P)
  - Visual highlight of currently selected project (✓ checkmark)
  - Loading states with spinner
  - Error handling with retry functionality
  - Empty state messages
  - Centered modal overlay (fixed width of 80 characters, centered vertically)

- **`styles.go`**: Lipgloss styles using GCP color palette

### Modified Files

#### 1. App Integration (`internal/ui/app.go`)

- Added `projectSelector` and `showProjectSelector` fields to App struct
- Initialized project selector in `NewApp()` with client and current project ID
- Added `ShowProjectSelectorOnStartup()` method for startup flow
- Updated `Init()` to show project selector on startup if configured
- Modified `Update()` to route messages to project selector when active
- Modified `View()` to render project selector overlay

#### 2. Command Palette (`internal/ui/components/commandpalette/commands.go`)

- Added "Switch Project" command to action commands
- Added new icon `IconProject = "◎"`
- Command appears at top of action commands list

#### 3. Command Handler (`internal/ui/app_commands.go`)

- Added `action:switch-project` case in `handleActionCommand()`
- Handler recreates project selector with current project ID and shows modal

#### 4. Navigation Logic (`internal/ui/app_navigation.go`)

- **`handleProjectSwitch()`**: Main project switching logic
  - Skips if selecting same project
  - Updates selected project and tracks in recent items
  - Clears all view instances
  - Syncs context
  - Reloads current view with new project
  - Closes modal

- **`clearAllViews()`**: Nils out all view instances and selected resources
  - Forces complete reload with new project data
  - Clears view stack and selected resources

- **`reloadCurrentView()`**: Recreates current view with new project ID
  - Returns to list views (not detail views) to avoid stale data
  - Handles all view types with appropriate fallbacks
  - Updates sidebar and sizes

#### 5. Rendering (`internal/ui/app_render.go`)

- Added `renderWithProjectSelector()` method
  - Overlays project selector on background
  - Centered horizontally and vertically
  - Preserves background content on sides
  - Uses ANSI-safe truncation helpers

#### 6. Startup Integration (`cmd/gcon/main.go`)

- Calls `ShowProjectSelectorOnStartup()` if no default project is configured
- Project selector appears immediately before any other UI

## Key Design Decisions

### 1. Modal Priority

Project selector has highest priority in message handling (before command palette). This ensures clean interaction when modal is open.

### 2. View State Management

When switching projects, all view instances are cleared and the current view is reloaded from the list level (not detail views). This ensures:
- No stale data from previous project
- Clean state transition
- Consistent user experience

### 3. Current Project Highlighting

The currently selected project is marked with a checkmark (✓) and cannot be selected again (modal just closes). This prevents unnecessary reloads.

### 4. Sidebar Behavior

Sidebar activation is controlled by `sidebarActive()` method which checks if `selectedProject != nil`. No explicit `SetActive()` calls needed - sidebar appears automatically when project is selected.

### 5. Filtering

Real-time filtering searches both project name and ID, making it easy to find projects by either identifier.

## Usage

### Via Command Palette

1. Press `:` or `Ctrl+K` to open command palette
2. Type "switch" or "project"
3. Select "Switch Project"
4. Modal appears with all accessible projects
5. Type to filter, use arrow keys or j/k to navigate
6. Press Enter to select, Esc to cancel

### On Startup (No Default Project)

1. Run `gcon` without `--project` flag and no `CLOUDSDK_CORE_PROJECT` set
2. Project selector modal appears automatically
3. Sidebar is hidden until project is selected
4. After selecting project, app initializes normally with sidebar

### Keyboard Shortcuts in Modal

- `↑`/`k` or `Ctrl-P`: Move up
- `↓`/`j` or `Ctrl-N`: Move down
- `Enter`: Select project
- `Esc`: Cancel
- Type to filter projects
- `r`: Retry loading (if error occurred)

## Testing

### Build & Tests

- Code compiles successfully
- All existing tests pass
- Linter passes (gofmt, unused checks)

### Manual Testing Required

1. **Command Palette Trigger**
   - Open command palette
   - Select "Switch Project"
   - Verify modal appears with projects

2. **Filtering**
   - Type partial project name
   - Verify real-time filtering
   - Clear with backspace

3. **Navigation**
   - Test all navigation keys (↑/↓, j/k, Ctrl-N/Ctrl-P)
   - Verify cursor wraps correctly

4. **Project Selection**
   - Select different project
   - Verify all views reload with new project data
   - Check breadcrumbs update
   - Check footer updates

5. **Same Project Selection**
   - Select currently active project
   - Verify modal just closes without reload

6. **Startup with No Project**
   ```bash
   unset CLOUDSDK_CORE_PROJECT
   gcon  # no --project flag
   ```
   - Verify modal appears immediately
   - Verify sidebar is hidden
   - After selection, verify normal app flow

7. **Error Handling**
   - Test with restricted permissions (if possible)
   - Verify error message displays
   - Test retry functionality

## Architecture Notes

### Message Flow

```
User presses : → Opens command palette
User selects "Switch Project" → handleActionCommand()
  → Sets showProjectSelector = true
  → Returns projectSelector.Init()
    → Async: loadProjects()
    → Returns projectsLoadedMsg

User selects project → ProjectSelectedMsg
  → handleProjectSwitch()
    → clearAllViews()
    → reloadCurrentView()
    → Closes modal
```

### State Management

- `showProjectSelector`: Boolean flag controlling modal visibility
- `projectSelector`: Component instance (recreated on each open with current project ID)
- `selectedProject`: Updated after successful selection
- All view instances: Cleared and recreated with new project ID

## Future Enhancements

### Deferred Features

1. **Footer Click Integration**: Phase 4 was not implemented
   - Clicking on `[project-id]` in footer to open selector
   - Requires mouse region tracking in footer
   - Not critical since command palette provides access

### Potential Improvements

1. **Project Favorites**: Allow starring frequently used projects
2. **Recent Projects**: Show recently accessed projects at top
3. **Project Search History**: Remember previous filter queries
4. **Project Metadata**: Show project number, state in modal
5. **Organization Grouping**: Group projects by organization/folder
6. **Quick Switch Keybinding**: Add dedicated global hotkey (e.g., `P`)

## Related Files

### New Files
- `internal/ui/components/projectselector/messages.go`
- `internal/ui/components/projectselector/projectselector.go`
- `internal/ui/components/projectselector/styles.go`

### Modified Files
- `internal/ui/app.go` - App struct, Init, Update, View
- `internal/ui/app_commands.go` - Command handler
- `internal/ui/app_navigation.go` - Project switching logic
- `internal/ui/app_render.go` - Modal overlay rendering
- `internal/ui/components/commandpalette/commands.go` - New command
- `cmd/gcon/main.go` - Startup integration

### Documentation Files
- `doc/2026-01-20-project-selector/TODO.md` - Task tracking
- `doc/2026-01-20-project-selector/Documentation.md` - This file

## Permissions Required

The project selector requires the following GCP permission:
- `cloudresourcemanager.projects.list` - To list accessible projects

If user lacks this permission, an error message is shown with retry option.

## Troubleshooting

### Modal Doesn't Appear
- Check that command palette is working (`:` key)
- Verify "Switch Project" appears in command list
- Check console for errors

### Projects Not Loading (Stuck on "Loading projects...")
- **Fixed Issue**: Ensure all messages (not just KeyMsg) are passed to project selector Update() method
- The app must pass `spinner.TickMsg` and internal messages through to the modal
- Check that App.Update() uses `default:` case to pass all messages when `showProjectSelector` is true

### Projects Not Loading (Error State)
- Verify GCP authentication: `gcloud auth application-default login`
- Check permissions: `cloudresourcemanager.projects.list`
- Try retry functionality (`r` key in error state)

### Filtering Not Working
- Verify text input is focused (should have cursor)
- Try clearing filter with backspace
- Check that projects exist matching filter

### Views Not Reloading
- Check that a different project was selected
- Verify `selectedProject` is updated in app state
- Check that view instances are cleared in `clearAllViews()`
