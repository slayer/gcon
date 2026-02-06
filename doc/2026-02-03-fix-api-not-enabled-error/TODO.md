# Fix: API Not Enabled Error Handling

## Task ID: 2026-02-03-fix-api-not-enabled-error

## Problem Description

When a GCP API is not enabled for a project, the application shows a loader indefinitely without displaying an error message to the user. This happens because:

1. The error classification in `internal/gcp/errors.go` doesn't distinguish "API not enabled" errors from generic "Permission denied" (403) errors
2. GCP returns specific error messages when an API is not enabled, but we're not detecting them

## Expected Behavior

When an API is not enabled:
- Loading state should stop
- Clear error message should be displayed: "API not enabled"
- Actionable hint should guide user to enable the API via Cloud Console
- User should be able to retry after enabling the API

## Root Cause Analysis

From code inspection:

1. **Error Classification** (`internal/gcp/errors.go:80-119`):
   - 403 errors are classified as either `ErrorQuotaExceeded` or `ErrorPermissionDenied`
   - No check for "API not enabled" specific messages
   - GCP returns messages like: "API [compute.googleapis.com] not enabled on project"

2. **Error Messages**:
   GCP API errors when API is not enabled typically contain:
   - `"has not been used in project"`
   - `"API [*.googleapis.com] not enabled"`
   - `"SERVICE_DISABLED"`
   - HTTP 403 status code
   - Details with link to enable the API

## Implementation Plan

### Step 1: Add ErrorAPINotEnabled Type
- Add new error code `ErrorAPINotEnabled` to the `ErrorCode` enum in `errors.go`
- This will allow specific handling and messaging for API disabled errors

### Step 2: Implement API Disabled Detection
- Add `containsAPIDisabledMessage()` function (similar to `containsQuotaMessage()`)
- Check for patterns:
  - "not enabled"
  - "has not been used"
  - "SERVICE_DISABLED"
  - "Enable it by visiting"

### Step 3: Update classifyAPIError()
- Add check for API disabled messages in 403 error handling
- Return `ErrorAPINotEnabled` with specific message and hint
- Hint should include:
  - Link to Cloud Console to enable API
  - Command to enable via gcloud CLI
  - Explanation of which API needs to be enabled

### Step 4: Update Error Display
- Verify `internal/ui/components/error_display.go` renders the new error type correctly
- Ensure icon and styling are appropriate for API configuration errors

### Step 5: Testing
- Manual test: Disable Compute Engine API in a test project
- Verify error message is displayed instead of infinite loader
- Verify "retry" after enabling API works correctly
- Test with multiple API types (Compute, Storage, Monitoring)

### Step 6: Documentation
- Update CLAUDE.md with new error type
- Document the error handling pattern for "API not enabled" scenarios

## Files to Modify

1. `internal/gcp/errors.go`
   - Add `ErrorAPINotEnabled` constant
   - Add `containsAPIDisabledMessage()` function
   - Update `classifyAPIError()` to check for API disabled messages

2. `internal/ui/components/error_display.go` (if needed)
   - Verify error rendering for new error type

## Verification Steps

- [ ] Create test GCP project or use existing one
- [ ] Disable Compute Engine API
- [ ] Run gcon and navigate to instances view
- [ ] Verify error message displays: "API not enabled" with link to enable
- [ ] Enable API via Cloud Console
- [ ] Press 'r' to retry
- [ ] Verify instances load successfully

## Edge Cases to Consider

1. Multiple APIs disabled (e.g., both Compute and Monitoring)
2. API enabled but IAM permissions still missing
3. Network errors vs API disabled errors
4. Service-specific error message variations

## Success Criteria

- [ ] No infinite loaders when API is disabled
- [ ] Clear "API not enabled" error message displayed
- [ ] Actionable hint with link to enable API
- [ ] Error includes which specific API needs to be enabled
- [ ] Retry works after enabling API
- [ ] Works for all GCP services (Compute, Storage, Monitoring, Logging)
