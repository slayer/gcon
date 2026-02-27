# IAM Policy View Redesign Documentation

## Summary

Redesigned the IAM Policy view from text-based viewport rendering to table-based views with row selection and editing capabilities. Users can now add/remove members to/from IAM role bindings directly from the TUI.

## Changes Made

### GCP Client (`internal/gcp/iam.go`)
- Added `rawPolicy` field to `IAMPolicy` struct for safe round-tripping
- Added `SetProjectIAMPolicy()` — writes policy back to GCP, preserving conditions and audit configs
- Added `AddMemberToRole()` / `RemoveMemberFromRole()` — read-modify-write with single retry on 409 etag conflict
- Added `ParseMemberType()` — parses `user:`, `serviceAccount:`, `group:`, `domain:`, `deleted:` prefixes
- Added `addMember()` / `removeMember()` helpers on IAMPolicy struct
- Added `toCRMPolicy()` for converting back to CRM Policy format
- Added `ErrEtagConflict` sentinel error

### IAM Policy View (`internal/ui/views/iam_policy.go`) — Full Rewrite
- Replaced `viewport.Model` tabs with `table.Model` for both tabs
- Embedded `TableClickDelegate` for mouse click handling
- Switched tab order: "By Member" is now the primary (first) tab
- **By Member table**: Type | Member | Roles columns
- **By Role table**: Role | Members | Preview columns
- Added detail overlay (Enter key) showing member's roles or role's members
- Added add/remove dialogs using `inputdialog.InputDialog` and `confirm.ConfirmDialog`
- Added action menu (`.` key) with context-dependent items
- Implemented `SetError()`, `UpdatePolicy()`, `HasTextInputFocused()`, `IsMenuOpen()`

### Message Types (`internal/ui/views/iam_messages.go`)
- `AddIAMBindingMsg` — requests adding a member to a role
- `RemoveIAMBindingMsg` — requests removing a member from a role
- `IAMPolicyUpdateResultMsg` — result of async policy update

### App Integration (`internal/ui/app.go`, `internal/ui/app_navigation.go`)
- Added message cases for new IAM policy messages
- Added `handleAddIAMBinding()`, `handleRemoveIAMBinding()`, `handleIAMPolicyUpdateResult()`
- Added `getIAMClient()` helper for finding IAM client from existing views
- Footer task registration for async policy updates

### Tests
- `internal/gcp/iam_test.go`: `TestParseMemberType`, `TestIAMPolicy_AddMember`, `TestIAMPolicy_RemoveMember`, `TestIAMPolicy_ToCRMPolicy`
- `internal/ui/views/iam_policy_test.go`: Table population, validators, overlay helpers, `HasTextInputFocused` states

## Architecture

```
User selects row → Enter key → Detail overlay
  ├── "a" key → Input dialog → AddIAMBindingMsg → App handler → GCP API → UpdatePolicy()
  └── "d" key → Confirm dialog → RemoveIAMBindingMsg → App handler → GCP API → UpdatePolicy()
```

The read-modify-write pattern for IAM policy updates:
1. Fetch current policy (includes etag)
2. Modify bindings in memory
3. Set policy with etag for optimistic concurrency
4. On 409 conflict: retry once from step 1

## Key Bindings

| Key | Action |
|-----|--------|
| Enter | Open detail overlay |
| a | Add member/role |
| d | Remove (in overlay, with confirmation) |
| . | Action menu |
| S | Sort menu |
| / | Filter |
| r | Refresh |
| Tab | Switch focus (tabs/table) |
| h/l | Switch tabs |
| Esc | Close overlay / Go back |
