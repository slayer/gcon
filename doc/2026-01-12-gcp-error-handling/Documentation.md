# GCP Error Handling - Documentation

## Summary

This change introduces user-friendly error handling for GCP API errors. Instead of showing raw API error strings, the application now displays clear messages with actionable hints.

## Changes Made

### New Components

#### `internal/gcp/errors.go`

Defines error types and parsing logic:

- `ErrorCode` enum for categorizing errors (Unauthenticated, PermissionDenied, NotFound, etc.)
- `GCPError` struct with user-friendly message, hint, operation context, and underlying error
- `ParseError()` function that inspects `*googleapi.Error` and classifies by HTTP status code
- Wrapper functions: `WrapListError()`, `WrapGetError()`, `WrapActionError()`

#### `internal/ui/components/error_display.go`

Provides `RenderError()` function that formats errors for the TUI:

- Displays icon based on error type (🔑 auth, 🚫 permission, ❓ not found, etc.)
- Shows operation context (what was attempted)
- Displays actionable hint
- Falls back to simple display for non-GCP errors

### Modified Files

- `internal/gcp/projects.go` - Uses `WrapListError` and `WrapGetError`
- `internal/gcp/compute.go` - Uses wrapper functions for all operations
- `internal/ui/views/projects.go` - Uses `components.RenderError()`
- `internal/ui/views/instances.go` - Uses `components.RenderError()`
- `internal/ui/views/instance_details.go` - Uses `components.RenderError()`

## Error Display Examples

### Permission Denied (403)

```
🚫 Permission denied

  Operation: list instances (my-project)

  Ensure your account has the required IAM permissions for this resource
  Press 'r' to retry
```

### Authentication Required (401)

```
🔑 Authentication required

  Operation: list projects

  Run 'gcloud auth application-default login' to authenticate
  Press 'r' to retry
```

### Resource Not Found (404)

```
❓ Resource not found

  Operation: get instance (my-vm)

  Verify the resource exists and the name/ID is correct
  Press 'r' to retry
```

## Architecture

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────────┐
│   GCP API Call  │────▶│  WrapError   │────▶│    GCPError     │
│  (returns err)  │     │  functions   │     │  (parsed info)  │
└─────────────────┘     └──────────────┘     └────────┬────────┘
                                                      │
                                                      ▼
┌─────────────────┐     ┌──────────────┐     ┌─────────────────┐
│   TUI Display   │◀────│ RenderError  │◀────│  View Update    │
│ (user sees msg) │     │  component   │     │  (err state)    │
└─────────────────┘     └──────────────┘     └─────────────────┘
```

## Testing

Tests are in `internal/gcp/errors_test.go`:

- Tests for each HTTP status code (401, 403, 404, 429, 503)
- Tests for quota detection in 403 errors
- Tests for network error detection
- Tests for error chain preservation (`errors.As()` works)
- Tests for nil error handling

Run tests:
```bash
make test
```

## Manual Testing

1. **Test authentication error**: Revoke credentials and try to list projects
   ```bash
   gcloud auth application-default revoke
   ./bin/gcon
   ```
   Expected: Shows 🔑 Authentication required with gcloud login hint

2. **Test permission error**: Try accessing a project without permissions
   Expected: Shows 🚫 Permission denied with IAM hint

3. **Test retry**: Press 'r' after any error to retry the operation
