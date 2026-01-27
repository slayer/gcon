# Bug Fix Summary: Label Editor "q" Key

## Issue
Typing "q" in the label editor text fields triggered the global quit action, making it impossible to enter labels containing the letter "q".

## Fix
Implemented `HasTextInputFocused()` method in `InstanceEditorView` to properly report when text input is focused, preventing global shortcuts from interfering with text entry.

## Changes
- **File**: `internal/ui/views/instance_editor.go`
  - Added `HasTextInputFocused()` method (10 lines)
- **File**: `internal/ui/views/instance_editor_test.go`
  - Added import for `labeledit` package
  - Added 2 comprehensive test functions covering all scenarios

## Test Results
- ✅ All 18 existing tests pass
- ✅ 2 new tests added with 5 sub-tests
- ✅ Lint passes with 0 issues
- ✅ Build succeeds

## Impact
- Users can now type any character in label fields without triggering global shortcuts
- No breaking changes to existing functionality
- Follows established design pattern from CLAUDE.md

## Verification Steps
1. Navigate to instance details
2. Press "l" to edit labels
3. Press "a" to add label
4. Type "queue" in key field → works correctly
5. Press Esc → exits editing, returns to navigation
6. Press "q" → quits app (as expected)
