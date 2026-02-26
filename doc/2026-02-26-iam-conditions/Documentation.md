# Fix IAM Policy Handling for Duplicate-Role Bindings

## Summary

GCP IAM v3 policies allow multiple bindings with the same role but different **IAM Conditions**. Previously, our `IAMBinding` model dropped condition information, causing three bugs:

1. `toCRMPolicy()` restored conditions via role-only first-match, so duplicate-role bindings got wrong/lost conditions
2. `addMember()`/`removeMember()` matched by role only, always operating on the first binding
3. UI table row IDs and lookups used role only, making duplicate-role bindings indistinguishable

## Changes

### Data Model (`internal/gcp/iam.go`)

- Added `ConditionTitle string` field to `IAMBinding`
- Added `BindingKey()` method — returns `"role"` (unconditioned) or `"role|title"` (conditioned)
- Added `ParseBindingKey()` function to split a composite key back into role + conditionTitle
- All internal matching now uses `(role, conditionTitle)` as composite key:
  - `iamPolicyFromAPI()` extracts condition title from CRM bindings
  - `addMember()` / `removeMember()` match on composite key
  - `toCRMPolicy()` restores conditions by matching `(role, conditionTitle)`
  - `AddMemberToRole()` / `RemoveMemberFromRole()` accept `conditionTitle` parameter
- Sort order changed from role-only to `(role, conditionTitle)` for determinism

### Messages (`internal/ui/views/iam_messages.go`)

- Added `ConditionTitle string` to both `AddIAMBindingMsg` and `RemoveIAMBindingMsg`

### App Handlers (`internal/ui/app_navigation.go`)

- Both IAM handlers thread `msg.ConditionTitle` through to the GCP client calls

### UI View (`internal/ui/views/iam_policy.go`)

- New `memberRole` struct carries `(role, conditionTitle)` pairs in the member entry
- `memberEntry.roles` changed from `[]string` to `[]memberRole`
- Role table uses `BindingKey()` as row IDs — differentiates duplicate-role rows
- Role table displays condition title in parentheses: `roles/viewer (Expires 2026)`
- Overlay context carries `conditionTitle` for accurate add/remove targeting
- `overlayBindKeys []string` parallel array tracks binding keys in "By Member" overlay
- `pendingConditionTitle` field ensures correct targeting through input/confirm dialog flow
- `findBinding()` matches by `BindingKey()` instead of role-only

### Tests (`internal/gcp/iam_test.go`, `internal/ui/views/iam_policy_test.go`)

New condition-specific tests:
- `TestIAMBinding_BindingKey` — key format with/without condition
- `TestParseBindingKey` — round-trip key parsing
- `TestIAMPolicyFromAPI_DuplicateRolesWithConditions` — both bindings preserved
- `TestIAMPolicy_AddMember_DuplicateRoles` — targets correct binding
- `TestIAMPolicy_RemoveMember_DuplicateRoles` — targets correct binding
- `TestIAMPolicy_ToCRMPolicy_PreservesConditions` — round-trip with add
- `TestIAMPolicy_ToCRMPolicy_MixedConditioned` — mixed conditioned/unconditioned

Existing tests updated for new `conditionTitle` parameter and `memberRole` struct.

## Design Decisions

- **Condition title as identity key**: GCP requires unique condition titles per role, making `(role, conditionTitle)` a stable composite key without modeling the full condition expression
- **UI doesn't create conditions**: Users can add/remove members from existing conditioned bindings, but new bindings created via the input dialog are always unconditioned (`conditionTitle=""`)
- **Pipe separator in BindingKey**: The `|` character is safe since neither GCP role names nor condition titles contain it
- **Backward compatible**: Unconditioned bindings (the vast majority) have `ConditionTitle=""` and `BindingKey()` returns just the role, so existing behavior is unchanged
