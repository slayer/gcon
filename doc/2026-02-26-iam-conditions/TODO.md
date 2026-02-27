# Fix IAM Policy Handling for Duplicate-Role Bindings (IAM Conditions)

## Task Description
GCP IAM allows multiple bindings with the same role but different conditions (IAM Conditions v3).
Our simplified `IAMBinding` model drops condition info, causing incorrect behavior when duplicate-role bindings exist.

## Implementation Plan

- [x] Step 1: Backend — `internal/gcp/iam.go`
  - Add `ConditionTitle` field to `IAMBinding`
  - Add `BindingKey()` helper method
  - Add `ParseBindingKey()` helper function
  - Update `iamPolicyFromAPI()` to extract condition titles
  - Update `addMember()` / `removeMember()` to match on (role, conditionTitle)
  - Update `toCRMPolicy()` to match on (role, conditionTitle) for condition restoration
  - Update `AddMemberToRole()` / `RemoveMemberFromRole()` to accept conditionTitle
- [x] Step 2: Messages — `internal/ui/views/iam_messages.go`
  - Add `ConditionTitle` to `AddIAMBindingMsg` and `RemoveIAMBindingMsg`
- [x] Step 3: App Handlers — `internal/ui/app_navigation.go`
  - Thread `msg.ConditionTitle` through both handler calls
- [x] Step 4: UI View — `internal/ui/views/iam_policy.go`
  - Add `pendingConditionTitle` field
  - Add `overlayBindKeys` for tracking binding keys in overlay
  - Update `memberEntry` to carry condition info per role via `memberRole` struct
  - Update table building, overlay, lookup, and message emission
  - Add `formatRoleWithCondition()` display helper
- [x] Step 5: Tests — `internal/gcp/iam_test.go`
  - Add `TestIAMBinding_BindingKey`
  - Add `TestParseBindingKey`
  - Add `TestIAMPolicyFromAPI_DuplicateRolesWithConditions`
  - Add `TestIAMPolicy_AddMember_DuplicateRoles`
  - Add `TestIAMPolicy_RemoveMember_DuplicateRoles`
  - Add `TestIAMPolicy_ToCRMPolicy_PreservesConditions`
  - Add `TestIAMPolicy_ToCRMPolicy_MixedConditioned`
  - Update existing tests for new conditionTitle parameter
  - Update `iam_policy_test.go` for `memberRole` struct
- [x] Step 6: Verify — lint & test (0 issues, all pass)
