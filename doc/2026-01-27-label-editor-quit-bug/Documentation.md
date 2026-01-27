# Fix: Label Editor "q" Key Triggers Quit

## Problem

When editing labels in the instance editor form, typing the letter "q" in a text input field would trigger the global quit action instead of typing the character into the field. This made it impossible to create labels with "q" in the key or value (e.g., "queue", "quality", "request").

## Root Cause

The `InstanceEditorView` did not implement the `HasTextInputFocused()` method from the `TextInputFocusable` interface. Without this method:

1. The app's global key handler at `internal/ui/app.go:416` couldn't detect when text input was focused
2. Character keys like "q" were processed as global shortcuts instead of being passed to the text input
3. The "q" key matched the Quit binding, causing the app to exit

## Solution

Implemented the `HasTextInputFocused()` method in `InstanceEditorView` to properly report when the label editor has a text input field focused:

```go
// HasTextInputFocused returns true if a text input field is currently focused
func (v *InstanceEditorView) HasTextInputFocused() bool {
    // Only when in form state and actively editing
    if v.state == stateForm && v.labelEditor != nil {
        return v.labelEditor.IsEditing()
    }
    return false
}
```

This method:
- Returns `true` when the view is in `stateForm` and the label editor is actively editing (text input focused)
- Returns `false` in all other states (loading, saving, diff preview, error)
- Delegates to `labelEditor.IsEditing()` which already tracks whether text inputs are focused

## How It Works

The app's key handling flow (in `internal/ui/app.go`):

1. **Line 416**: Check if any view has text input focused via `hasTextInputFocused()`
2. **If true**: Pass the key message directly to the view, skipping global shortcuts
3. **If false**: Process global key bindings like "q" for quit, "?" for help, etc.

With the fix:
- When user presses "a" to add a label or "e" to edit → `HasTextInputFocused()` returns `true`
- User can type "q", "?", "/" etc. in the text fields
- When user presses Esc or Enter to exit edit mode → `HasTextInputFocused()` returns `false`
- Global shortcuts work normally again

## Files Changed

### Modified
- `internal/ui/views/instance_editor.go`: Added `HasTextInputFocused()` method

### Test Coverage
- `internal/ui/views/instance_editor_test.go`: Added comprehensive tests
  - `TestInstanceEditorView_HasTextInputFocused`: Tests all state combinations
  - `TestInstanceEditorView_GlobalQuitNotBlockedWhenNotEditing`: Integration test

## Testing

All existing tests pass, plus new tests verify:
- Method returns `false` in all non-editing states (loading, saving, diff, error)
- Method returns `false` when in form state but not editing (navigation mode)
- Method returns `true` when in form state and actively editing
- Integration scenario: navigation mode → add label → editing mode detected

```bash
go test ./internal/ui/views/... -v -run TestInstanceEditor
# PASS (18 tests)

make lint
# 0 issues
```

## Design Pattern

This follows the established pattern documented in `CLAUDE.md`:

> ### Text Input Focus and Global Keys
>
> Views containing forms must implement `TextInputFocusable`:
>
> ```go
> func (v *MyView) HasTextInputFocused() bool {
>     return v.form.HasTextInputFocused()
> }
> ```

The pattern ensures:
- Character keys go to text inputs when appropriate
- Global shortcuts work normally when not editing
- Consistent behavior across all form-based views
- No need to modify app-level key handling for each new view

## Verification

To manually verify the fix:
1. Run the app and navigate to an instance
2. Press "l" to edit labels
3. Press "a" to add a new label
4. Type "queue" in the key field → should work without quitting
5. Type "quality" in the value field → should work
6. Press Esc to cancel → should exit label editing, not quit app
7. Press "q" in navigation mode → should quit the app

## Related Components

- `internal/ui/components/labeledit/labeledit.go`: The label editor component
  - Already has `IsEditing()` method that tracks text input focus
- `internal/ui/components/forms/form.go`: Form components also implement this pattern
- `internal/ui/views/form_demo.go`: Example implementation

## Future Improvements

Other views using text input components should verify they implement `HasTextInputFocused()`:
- Search/filter inputs in list views (already handle this via input blur)
- Any future form-based editing views
- Command palette (already handles this correctly)
