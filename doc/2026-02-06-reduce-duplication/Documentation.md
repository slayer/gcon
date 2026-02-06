# Reduce UI Code Duplication and Unify Patterns

## Summary

Eliminated ~515 lines of duplicated code across 32 files by extracting shared patterns into reusable helpers and base types. This makes it significantly easier to add new views with less boilerplate.

## Changes Made

### Phase 1: Spinner Factory Function
- Added `NewGCPSpinner()` to `internal/ui/components/spinner.go`
- Replaced 20+ inline 3-line spinner creation patterns with a single function call
- Ensures consistent GCP blue styling (#4285F4) across all views

### Phase 2: Shared renderLoading/renderSaving Helpers
- Added `renderLoading()` and `renderSaving()` to `internal/ui/views/helpers.go`
- Removed 15+ identical per-view `renderLoading` methods
- Removed 3 per-view `renderSaving` methods from creation views

### Phase 3: Form Sizing Constants
- Added `formWidthPadding` and `formHeightPadding` constants to `helpers.go`
- Replaced magic numbers (-4, -4) in form sizing across 4 views

### Phase 4: Inline Error Display Function
- Added `RenderInlineError()` to `internal/ui/components/error_display.go`
- Provides form-appropriate error display (no "retry" hint)
- Replaced inline error styling in 3 creation views
- Added table-driven tests in `error_display_test.go`

### Phase 5: TableClickDelegate
- Added `TableClickDelegate` struct to `helpers.go`
- Embeddable struct that delegates Clickable interface (3 methods) to `table.Model`
- Removed 21 lines of boilerplate from each of 7 list views (projects, instances, disks, snapshots, images, buckets, objects)
- Added compile-time interface checks in `helpers_test.go`

### Phase 6: CreateViewBase
- Created `internal/ui/views/create_view_base.go` with `CreateViewBase` struct
- Provides shared lifecycle: state machine, spinner, form sizing, error display, cancel handling
- Refactored `snapshot_create.go`, `disk_create.go`, `image_create.go` to embed it
- Each view shrank by ~80-110 lines while retaining all unique logic
- `bucket_create.go` intentionally not refactored (more complex lifecycle with diff viewer)

## Technical Details

### CreateViewBase Architecture
```
CreateViewBase (embedded)
├── State machine (form/saving)
├── Spinner (GCP blue)
├── Form (forms.Form)
├── Error handling (SetError)
├── Context/sizing propagation
├── View rendering (form + error / saving spinner)
├── HandleBaseUpdate (spinner ticks, cancel-during-saving)
└── UpdateForm (form message delegation)

Concrete View (embedder)
├── buildForm() — unique form configuration
├── Update() — view-specific messages, delegates to base
├── handleSubmit() — unique data extraction
└── Accessor methods (GetDiskName, etc.)
```

### Linter Fix
During Phase 6, the linter flagged `switch msg.(type)` + `msg.(xxxErrorMsg).err` patterns (errcheck + gocritic). Fixed by using `switch msg := msg.(type)` with proper variable binding.

## Testing
- All existing tests pass without modification (except test files that referenced old internal state constants, which were updated)
- New tests: `error_display_test.go`, `helpers_test.go`
- `make lint` — 0 issues
