# Project Selection Dialog

## Task Description

Replace the full-screen `ProjectsView` with a modal `ProjectDialog` that:
- Shows on startup when no default project is detected
- Accessible via command palette ("Switch Project" command)
- Accessible via click on project section in footer
- Marks the currently selected project with a visual indicator
- Shows error inline with retry option if project fetch fails
- Resets all views when project changes

## Implementation Plan

### Phase 1: Create ProjectDialog Component
- [x] Create `internal/ui/components/projectdialog/projectdialog.go`
- [x] Implement project list with search/filter
- [x] Add loading state with spinner
- [x] Add error state with retry option
- [x] Mark currently selected project with checkmark
- [x] Emit `ProjectDialogSelectedMsg` and `ProjectDialogClosedMsg`

### Phase 2: Integrate into App
- [x] Add dialog state to app.go (`showProjectDialog`, `projectDialog`)
- [x] Modify startup flow - show dialog when no project detected
- [x] Create `resetAppState()` to nil all views on project change
- [x] Handle dialog messages in App.Update()
- [x] Render dialog as overlay (like command palette)

### Phase 3: Add Access Points
- [x] Add "Switch Project" command to command palette
- [x] Add click handling to footer Right3 (project section)

### Phase 4: Cleanup
- [x] Remove or deprecate `ProjectsView` (kept for backward compat, but unused)
- [x] Update sidebar to remove projects entry if needed (N/A - not in sidebar)
- [x] Update tests

## Requirements
- Modal dialog pattern (like ActionMenu)
- Search/filter projects
- Loading spinner during API calls
- Error inline with retry
- Current project marked with checkmark
- Escape to close (if project already selected)
- On startup without project: dialog is mandatory (no escape)

## Testing
- [x] Test dialog opens on startup when no project
- [x] Test dialog opens from command palette
- [x] Test dialog opens from footer click
- [x] Test project selection resets all views
- [x] Test error handling with retry
- [x] Test search/filter functionality
