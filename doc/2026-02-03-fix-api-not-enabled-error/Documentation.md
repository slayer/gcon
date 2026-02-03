# Fix: API Not Enabled Error Handling

## Task ID: 2026-02-03-fix-api-not-enabled-error

## Summary

Fixed a bug where the application showed an infinite loader when GCP APIs were not enabled for a project, instead of displaying a clear error message. The fix introduces specific detection and user-friendly messaging for "API not enabled" errors.

## Problem

When a user tried to access resources (e.g., Compute Engine instances) in a project where the required API was not enabled, the UI would:
- Show a loading spinner indefinitely
- Never display an error message
- Give no indication of what went wrong

This left users confused and unable to understand that they needed to enable the API.

## Solution

### 1. Added New Error Type

Added `ErrorAPINotEnabled` to the `ErrorCode` enum in `internal/gcp/errors.go`:

```go
const (
    ErrorUnknown            ErrorCode = iota
    ErrorUnauthenticated              // 401 - credentials invalid or expired
    ErrorPermissionDenied             // 403 - missing IAM permissions
    ErrorAPINotEnabled                // 403 - API not enabled for project (NEW)
    ErrorNotFound                     // 404 - resource doesn't exist
    ErrorRateLimited                  // 429 - too many requests
    ErrorQuotaExceeded                // 429 with quota message
    ErrorServiceUnavailable           // 503 - GCP service issues
    ErrorNetwork                      // connection/network errors
)
```

### 2. API Disabled Detection Function

Implemented `containsAPIDisabledMessage()` to detect GCP's "API not enabled" error patterns:

```go
func containsAPIDisabledMessage(msg string) bool {
    msgLower := strings.ToLower(msg)
    keywords := []string{
        "api has not been used",
        "not enabled",
        "service_disabled",
        "enable it by visiting",
        "api is not enabled",
    }
    for _, kw := range keywords {
        if strings.Contains(msgLower, kw) {
            return true
        }
    }
    return false
}
```

### 3. Updated Error Classification

Modified `classifyAPIError()` to check for API disabled errors before other 403 error types:

```go
case 403:
    // Check for API not enabled errors first (most specific)
    if containsAPIDisabledMessage(apiErr.Message) {
        return ErrorAPINotEnabled,
            "API not enabled",
            "Enable the required API in Cloud Console or run: gcloud services enable <api-name>"
    }
    // Then check for quota and permission errors...
```

The order is important: API not enabled errors are more specific than generic permission denied errors.

### 4. Updated Error Display

Added icon for API not enabled errors in `internal/ui/components/error_display.go`:

```go
case gcp.ErrorAPINotEnabled:
    return "⚙️"  // Settings/configuration icon
```

## Changes Made

### Files Modified

1. **internal/gcp/errors.go**
   - Added `ErrorAPINotEnabled` constant
   - Added `containsAPIDisabledMessage()` function
   - Updated `classifyAPIError()` to detect API disabled errors

2. **internal/ui/components/error_display.go**
   - Added icon case for `ErrorAPINotEnabled`

3. **internal/gcp/errors_test.go**
   - Added test case for "403 API not enabled" error
   - Added `TestContainsAPIDisabledMessage()` function with comprehensive test cases

## Error Messages

### Before
- Infinite loading spinner
- No error message
- No indication of the problem

### After
```
⚙️ API not enabled

Operation: list instances (my-project)

Enable the required API in Cloud Console or run: gcloud services enable <api-name>
Press 'r' to retry
```

## Testing

### Unit Tests

Added comprehensive tests in `internal/gcp/errors_test.go`:

1. **API Error Classification Test**
   - Tests that 403 errors with "not enabled" message are classified as `ErrorAPINotEnabled`
   - Verifies hint contains "gcloud services enable"

2. **API Disabled Message Detection Test**
   - Tests various GCP error message patterns:
     - `"API [compute.googleapis.com] not enabled on project [my-project]"`
     - `"compute.googleapis.com API has not been used in project 123456789"`
     - `"Service compute.googleapis.com is not enabled for the project"`
     - `"SERVICE_DISABLED"`
     - `"Enable it by visiting https://console.cloud.google.com/..."`
     - `"API is not enabled"`
   - Verifies false positives don't trigger (permission denied, quota, etc.)

All tests pass:
```bash
$ go test ./internal/gcp/... -v
PASS
ok      github.com/slayer/gcon/internal/gcp     0.334s
```

### Manual Testing Steps

To manually verify the fix:

1. Create or use a test GCP project
2. Ensure Compute Engine API is **disabled**:
   ```bash
   gcloud services disable compute.googleapis.com --project=<project-id>
   ```
3. Run gcon and select the project
4. Navigate to the Compute Engine instances view
5. **Expected**: Error message displays:
   - Icon: ⚙️
   - Message: "API not enabled"
   - Hint: "Enable the required API in Cloud Console or run: gcloud services enable <api-name>"
   - "Press 'r' to retry"
6. Enable the API:
   ```bash
   gcloud services enable compute.googleapis.com --project=<project-id>
   ```
7. Press 'r' to retry
8. **Expected**: Instances list loads successfully

## Error Detection Patterns

The fix detects the following GCP error message patterns (case-insensitive):

| Pattern | Example Error Message |
|---------|----------------------|
| `"api has not been used"` | "compute.googleapis.com API has not been used in project 123456789" |
| `"not enabled"` | "API [compute.googleapis.com] not enabled on project [my-project]" |
| `"service_disabled"` | "SERVICE_DISABLED" |
| `"enable it by visiting"` | "Enable it by visiting https://console.cloud.google.com/..." |
| `"api is not enabled"` | "API is not enabled" |

## Benefits

1. **Clear Error Messages**: Users immediately understand the problem
2. **Actionable Guidance**: Specific instructions on how to enable the API
3. **Consistent UX**: Same error handling pattern as other error types
4. **Proper State Management**: Loading state stops when error occurs
5. **Easy Recovery**: Users can retry after enabling the API

## Edge Cases Handled

1. **Multiple 403 Error Types**: API not enabled errors are distinguished from:
   - Permission denied (IAM issues)
   - Quota exceeded
   - Other 403 errors

2. **Order of Detection**: API disabled is checked **before** generic permission denied to ensure proper classification

3. **Case Insensitivity**: Error message matching is case-insensitive to handle variations in GCP error messages

4. **Multiple APIs**: Works for all GCP APIs:
   - Compute Engine API
   - Cloud Storage API
   - Cloud Monitoring API
   - Cloud Logging API
   - etc.

## Related Code

The error handling flow:

```
GCP API Call (compute.go, storage.go, etc.)
    ↓
Error Returned (403 with "API not enabled" message)
    ↓
WrapListError/WrapGetError/WrapActionError (errors.go)
    ↓
ParseError (errors.go)
    ↓
classifyAPIError (errors.go)
    ↓
containsAPIDisabledMessage (errors.go) ← NEW
    ↓
Return GCPError with ErrorAPINotEnabled code
    ↓
View receives instancesErrorMsg/etc
    ↓
View sets loading=false, err=msg.err
    ↓
RenderError (error_display.go)
    ↓
Display ⚙️ icon with error message and hint
```

## Future Improvements

Potential enhancements for the future:

1. **Extract API Name**: Parse the exact API name from the error message and include it in the hint
   - Current: "gcloud services enable <api-name>"
   - Enhanced: "gcloud services enable compute.googleapis.com"

2. **Direct Link to Enable**: Include a clickable link to the Cloud Console page to enable the API

3. **Auto-Enable Option**: Prompt user to auto-enable the API via gcloud CLI

4. **Project-Level Warning**: Show a warning in the project selector if common APIs are disabled

## Conclusion

This fix resolves the infinite loader bug and provides a much better user experience when GCP APIs are not enabled. Users now get clear, actionable error messages that guide them to enable the required APIs and retry their operations.
