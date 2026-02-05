# Summary: Fix API Not Enabled Error Handling

## Issue
When GCP APIs (e.g., Compute Engine API) were not enabled for a project, the application showed an infinite "Loading..." spinner instead of displaying a clear error message. Users had no indication of what was wrong or how to fix it.

## Root Causes Discovered

### 1. Primary Issue: Missing API Disabled Detection
The error parsing in `internal/gcp/errors.go` did not detect "API not enabled" errors. These errors come from GCP with 403 status codes and specific message patterns, but they were being classified as generic "Permission denied" errors.

### 2. Secondary Issues Found During Investigation
- **Window Size Not Detected**: In tmux/screen environments (`screen-256color`), Bubble Tea's WindowSizeMsg was never received, causing the app to stay stuck on "Loading..."
- **Nil Pointer Crash**: After selecting a project from the project selector, `projectView` was set to nil but still accessed, causing a crash
- **Missing Nil Check**: `updateViewSizes()` called `projectView.SetContext()` without checking for nil

## Solutions Implemented

### 1. API Not Enabled Detection (`internal/gcp/errors.go`)
- Added `ErrorAPINotEnabled` error code to the `ErrorCode` enum
- Implemented `containsAPIDisabledMessage()` function to detect patterns:
  - "api has not been used"
  - "not enabled"
  - "service_disabled"
  - "enable it by visiting"
  - "api is not enabled"
  - "is not enabled"
  - "has not been enabled"
- Updated `classifyAPIError()` to check for API disabled errors before generic permission errors
- Added fallback check in `ParseError()` for non-googleapi.Error wrapped errors

### 2. Error Display (`internal/ui/components/error_display.go`)
- Added ⚙️ icon for `ErrorAPINotEnabled` errors
- Error message: "API not enabled"
- Helpful hint: "Enable the required API in Cloud Console or run: gcloud services enable <api-name>"

### 3. Window Size Fallback (`internal/ui/app.go`)
- Added fallback initialization in `Init()` to first try `term.GetSize` and, if no WindowSizeMsg arrives, fall back to a default 160x50 size
- This ensures the app works reliably in tmux/screen environments where WindowSizeMsg may never be sent

### 4. Project Selector Navigation Fix (`internal/ui/app_navigation.go`)
- Fixed `reloadCurrentView()` to switch from ViewProjects to ViewInstances after project selection
- Prevents staying on a cleared projectView

### 5. Nil Check Fix (`internal/ui/app.go`)
- Added nil check for `projectView` in `updateViewSizes()` before calling `SetContext()`
- Consistent with all other view checks

## Testing

### Unit Tests
- Added test case for "403 API not enabled" error classification
- Added `TestContainsAPIDisabledMessage()` with 9 test cases covering various GCP error patterns
- All existing tests still pass

### Manual Testing
Verified with Compute Engine API disabled:
- ✅ Project selector loads and displays projects
- ✅ After selecting project, switches to instances view
- ✅ Shows "⚙️ API not enabled" error message instead of infinite loading
- ✅ Error includes helpful hint about enabling the API
- ✅ Retry ('r' key) works after enabling API

## Files Changed

### Core Fix
- `internal/gcp/errors.go` - Added API disabled detection logic
- `internal/gcp/errors_test.go` - Added comprehensive tests
- `internal/ui/components/error_display.go` - Added icon for API not enabled

### Bug Fixes
- `internal/ui/app.go` - Window size fallback + nil check for projectView
- `internal/ui/app_navigation.go` - Fixed project selector navigation

### Debug Infrastructure (Optional)
- `internal/debug/debug.go` - Created reusable debug logging package (logs to ./gcon-debug.log)

## Benefits

1. **Clear Error Messages**: Users immediately understand the problem
2. **Actionable Guidance**: Specific instructions on how to enable the API
3. **Better UX**: No more infinite loaders - errors are displayed promptly
4. **Consistent Error Handling**: Same pattern as other error types
5. **Works in All Environments**: Fixed tmux/screen compatibility

## Error Message Example

Before:
```
  Loading...
  (infinite spinner, no indication of problem)
```

After:
```
  ⚙️ API not enabled

  Operation: list instances (my-project)

  Enable the required API in Cloud Console or run: gcloud services enable <api-name>
  Press 'r' to retry
```

## Deployment

The fix is backward compatible and requires no configuration changes. Users will automatically get better error messages when APIs are not enabled.
