# Task: Authenticated User Identity Detection

## Task ID
2026-01-17-auth-identity

## Description
Add functionality to detect and display the authenticated user's email/identity from Google Cloud Application Default Credentials (ADC). Support both user accounts (via gcloud) and service accounts.

## Implementation Steps

- [ ] Phase 1: Add Identity Retrieval Function
  - [ ] Create `internal/config/identity.go` with `GetAuthenticatedIdentity()`
  - [ ] Implement `extractServiceAccountEmail()` helper
  - [ ] Write tests in `internal/config/identity_test.go`

- [ ] Phase 2: Update GCP Client
  - [ ] Add `authenticatedIdentity` field to Client struct
  - [ ] Populate identity during client initialization
  - [ ] Add getter method `GetAuthenticatedIdentity()`

- [ ] Phase 3: Display in UI Footer
  - [ ] Add `authenticatedIdentity` field to App struct
  - [ ] Implement email truncation helper
  - [ ] Update `syncFooter()` to display identity in Right3 slot

- [ ] Phase 4: Testing
  - [ ] Write tests for identity retrieval
  - [ ] Write tests for footer identity display
  - [ ] Run full test suite

- [ ] Phase 5: Verification
  - [ ] Test with user credentials
  - [ ] Test with service account credentials
  - [ ] Test truncation with long emails
  - [ ] Test with no credentials
  - [ ] Run linter

## Notes
- Identity detection is informational, not critical
- Failure to detect identity should NOT prevent app from working
- Smart truncation preserves username start and domain end
