# Refactor app.go - Make it Cleaner

**Date**: 2026-01-16
**Branch**: `2026-01-16-refactor-app-module`

## Objective

Break down the large `internal/ui/app.go` file (1529 lines) into smaller, more maintainable modules using a moderate split approach.

## Current Structure Analysis

`app.go` currently handles:
- Core App struct and initialization (~155 lines)
- Navigation logic and view routing (~380 lines)
- Message handling in Update() (~300 lines)
- View rendering (header, sidebar, content, footer) (~400 lines)
- Command palette handling (~90 lines)
- Footer sync and task status (~130 lines)
- Utility functions (width calculations, truncation) (~70 lines)

## Refactoring Plan

### 1. Extract Navigation Handler (`app_navigation.go`)
**Lines to extract**: ~400 lines
- `handleSidebarNavigation()`
- `updateSidebarActiveView()`
- View stack management logic
- View initialization for each type
- Navigation-related message handlers:
  - `ProjectSelectedMsg`
  - `InstanceSelectedMsg`
  - `DiskSelectedMsg`
  - `BucketSelectedMsg`
  - etc.

### 2. Extract Render Manager (`app_render.go`)
**Lines to extract**: ~450 lines
- All View() rendering logic
- `renderHeader()`
- `renderFooter()`
- `renderCurrentView()`
- `renderWithSidebar()`
- `renderWithCommandPalette()`
- `renderPlaceholder()`
- Width/truncation utilities:
  - `truncateRight()`
  - `truncateLeft()`
  - `truncateToHeight()`

### 3. Extract Command Handler (`app_commands.go`)
**Lines to extract**: ~140 lines
- `openCommandPalette()`
- `handleCommandSelected()`
- `handleNavigationCommand()`
- `handleActionCommand()`
- `handleRecentCommand()`

### 4. Extract Footer Manager (`app_footer.go`)
**Lines to extract**: ~160 lines
- `syncFooter()`
- `renderTaskStatus()`
- Task status styles
- Color utilities:
  - `colorFromString()`
  - `hueToRGB()`

### 5. Keep in `app.go` (Core)
**Remaining**: ~380 lines
- App struct definition
- `NewApp()`
- `Init()`
- Core `Update()` method (routing to handlers)
- `updateCurrentView()`
- `getCurrentViewModel()`
- `updateViewSizes()`
- `syncContext()`
- Helper methods: `sidebarActive()`, `isViewMenuOpen()`, `toggleFocus()`, `cleanup()`
- Task management: `startTask()`, `finishTask()`

## Implementation Steps

1. ✅ Create branch and documentation
2. Create `app_navigation.go` with navigation logic
3. Create `app_render.go` with rendering logic
4. Create `app_commands.go` with command palette handlers
5. Create `app_footer.go` with footer sync and task status
6. Refactor `app.go` to import and use extracted handlers
7. Run tests to ensure no breakage
8. Run linter
9. Update documentation

## Expected Result

After refactoring:
- `app.go`: ~380 lines (core app logic)
- `app_navigation.go`: ~400 lines (navigation)
- `app_render.go`: ~450 lines (rendering)
- `app_commands.go`: ~140 lines (commands)
- `app_footer.go`: ~160 lines (footer/status)

## Testing Strategy

- Run existing tests: `make test`
- Run linter: `make lint`
- Manual testing: Launch app and verify:
  - Project selection works
  - Navigation between views works
  - Command palette works
  - Sidebar navigation works
  - Footer displays correctly
  - Task status shows correctly

## Notes

- All extracted files will be in the same package (`ui`)
- Methods will remain as App methods (receiver `*App`)
- No changes to public API or behavior
- Pure code organization refactor
