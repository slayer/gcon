# IAM Policy View Redesign: Table Format + Member Editing

## Task Description
Convert IAM Policy view from text-based viewport to table-based view with row selection, add ability to add/remove members to/from role bindings.

## Implementation Plan

- [x] Step 1: GCP Client — Add SetIamPolicy + Helpers (`internal/gcp/iam.go`)
  - Store raw CRM Policy in IAMPolicy for round-tripping
  - Add `ParseMemberType(member)` helper
  - Add `SetProjectIAMPolicy(ctx, projectID, policy)`
  - Add `AddMemberToRole(ctx, projectID, role, member)` with retry on 409
  - Add `RemoveMemberFromRole(ctx, projectID, role, member)` with retry on 409

- [x] Step 2: Redesign IAM Policy View (`internal/ui/views/iam_policy.go`)
  - Replace viewports with `memberTable` + `roleTable`
  - Embed `TableClickDelegate`
  - Switch tab order: "By Member" first, "By Role" second
  - By Member columns: Type | Member | Roles
  - By Role columns: Role | Members | Preview

- [x] Step 3-4: Overlay + Add/Remove Dialogs
  - Enter on row opens detail overlay (member's roles or role's members)
  - `a` to add via InputDialog
  - `d` to remove via ConfirmDialog
  - Overlay with cursor navigation (j/k)

- [x] Step 5: Messages + App Integration (`iam_messages.go`, `app.go`, `app_navigation.go`)
  - AddIAMBindingMsg, RemoveIAMBindingMsg, IAMPolicyUpdateResultMsg
  - App handlers with async GCP calls
  - Footer task registration

- [x] Step 6-7: Key bindings + Action Menu + HasTextInputFocused + SetError
  - Action menu with context-dependent items
  - HasTextInputFocused for filter, input dialog, confirm dialog
  - SetError for app-driven error propagation

- [x] Step 8: Tests
  - `internal/gcp/iam_test.go`: ParseMemberType, policy round-trip
  - `internal/ui/views/iam_policy_test.go`: table population, HasTextInputFocused

- [x] Step 9: Documentation
  - Update key bindings in `.claude/rules/key-bindings.md`
  - Update CLAUDE.md Implemented Features
  - Create Documentation.md
