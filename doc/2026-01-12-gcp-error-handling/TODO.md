# GCP Error Handling Improvements

## Task Description

Add GCP-specific error parsing to provide user-friendly messages with actionable hints for common errors like permission denied (403), authentication failures (401), and rate limiting (429).

## Implementation Plan

- [x] Create branch and documentation
- [x] Create `internal/gcp/errors.go` - error types and parsing
- [x] Create `internal/gcp/errors_test.go` - tests
- [x] Update `internal/gcp/projects.go` - add error wrapping
- [x] Update `internal/gcp/compute.go` - add error wrapping
- [x] Create `internal/ui/components/error_display.go` - UI rendering
- [x] Update views to use new error display
- [x] Run tests and linter
- [ ] Create PR

## Error Codes to Handle

| HTTP Code | Type | Message | Hint |
|-----------|------|---------|------|
| 401 | Unauthenticated | Authentication required | Run 'gcloud auth application-default login' |
| 403 | Permission denied | Permission denied | Check IAM permissions |
| 404 | Not found | Resource not found | Verify resource exists |
| 429 | Rate limited | Too many requests | Wait and retry |
| 503 | Service unavailable | Service temporarily unavailable | Try again later |

## Files Changed

- `internal/gcp/errors.go` (new) - error types, parsing, and wrapper functions
- `internal/gcp/errors_test.go` (new) - comprehensive tests for error handling
- `internal/gcp/projects.go` (modified) - added error wrapping
- `internal/gcp/compute.go` (modified) - added error wrapping to all operations
- `internal/ui/components/error_display.go` (new) - UI error rendering with icons
- `internal/ui/views/projects.go` (modified) - use new error display
- `internal/ui/views/instances.go` (modified) - use new error display
- `internal/ui/views/instance_details.go` (modified) - use new error display
