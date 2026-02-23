# IAM & Admin Feature

## Phase 1: GCP Client Layer
- [x] Create `internal/gcp/iam.go` with IAMClient, domain types, API methods
- [x] Create `internal/gcp/iam_test.go` with mock interface and tests
- [x] Add sentinel error to `internal/ui/errors/errors.go`

## Phase 2: Messages + Sidebar
- [x] Create `internal/ui/views/iam_messages.go` with all IAM message types
- [x] Update `internal/ui/components/sidebar/menu.go` with IAM ViewTypes and menu items

## Phase 3: Service Accounts List View
- [x] Create `internal/ui/views/service_accounts.go`
- [x] Wire up in app.go (ViewType enum, struct fields, getCurrentViewModel, Update handlers)
- [x] Wire up in app_render.go (renderCurrentView, breadcrumbs)
- [x] Wire up in app_navigation.go (navigation, clearAllViews, updateSidebarActiveView, reloadCurrentView)
- [x] Create `internal/ui/views/service_accounts_test.go`

## Phase 4: Service Account Details
- [x] Create `internal/ui/views/service_account_details.go` (tabbed: Details/Keys)

## Phase 5: Service Account Create
- [x] Create `internal/ui/views/service_account_create.go` (CreateViewBase)

## Phase 6: App Handlers (IAM actions)
- [x] Add handlers for delete, enable/disable, key create/delete
- [x] Wire up result messages in Update()

## Phase 7: IAM Policy View
- [x] Create `internal/ui/views/iam_policy.go` (tabbed: By Role / By Member)
- [x] Add filter (`/`) for roles and members with live search

## Phase 8: Custom Roles
- [x] Create `internal/ui/views/custom_roles.go` (list)
- [x] Create `internal/ui/views/custom_role_details.go` (tabbed: Details/Permissions)

## Phase 9: Polish
- [x] Run `make lint` and fix issues
- [x] Run `make test`
- [x] Update CLAUDE.md with new features
- [x] Update key-bindings.md
