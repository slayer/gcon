# gh-dash Patterns Implementation

## Task Description
Adopt valuable patterns from gh-dash TUI into gcon to improve code organization, UX, and performance.

## Implementation Plan

### Phase 1: Centralized Context (High Value, Low Effort)
- [x] Create `internal/ui/context/context.go` with ProgramContext struct
- [x] Add shared dimensions (ScreenWidth, ScreenHeight, ContentWidth, ContentHeight)
- [x] Add shared styles reference
- [x] Add StartTask callback for async operations
- [ ] Update App to propagate context to all views (deferred - requires larger refactor)
- [ ] Update views to use context instead of individual SetSize calls (deferred)

### Phase 2: Task Tracking System (High Value, Medium Effort)
- [x] Create Task struct with states in `internal/ui/context/context.go`
- [x] Add task map to ProgramContext for tracking active operations
- [x] Create messages for task lifecycle (TaskStartedMsg, TaskFinishedMsg, TaskClearMsg)
- [ ] Update async operations to use task system (deferred - requires view changes)
- [ ] Implement auto-clear after task completion (deferred)

### Phase 3: Table Enhancements (High Value, Medium Effort)
- [x] Add Column struct with Title, Width, Hidden, Grow, ComputedWidth fields
- [x] Add NewWithColumns constructor for enhanced columns
- [x] Implement width caching to prevent recalculation on every render
- [x] Add Grow flag support for flexible column widths
- [x] Add SetColumnHidden method for runtime visibility control
- [x] Add loading and empty state handling (SetLoading, SetEmptyText)

### Phase 4: Enhanced Footer (Medium Value, Low Effort)
- [x] Create `internal/ui/components/footer/footer.go`
- [x] Support left/right sections with dynamic content
- [x] Show active task indicator with elapsed time
- [x] Dynamic spacing to fill width
- [x] Confirm quit mode
- [x] FormatResourceCount and FormatLastRefresh helpers

### Future Improvements (Not in this PR)
- Fuzzy autocomplete for filtering
- API response caching with otter
- Config-based keybindings
- Auto-refresh polling
- Integrate new context into existing views

## Decisions
- Created foundation components without modifying existing views
- New components are opt-in and backward compatible
- Existing table.New() continues to work unchanged
- Views can gradually adopt new patterns

## Testing Strategy
- [x] Unit tests for context package (6 tests)
- [x] Unit tests for enhanced table component (10 tests)
- [x] Unit tests for footer component (9 tests)
- [x] All existing tests pass
- [x] Linter passes
