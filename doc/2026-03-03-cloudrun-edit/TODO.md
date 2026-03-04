# Cloud Run Service Edit and Deploy View

## Task Description
Add form-based editor view for editing existing Cloud Run services and creating new ones. Includes diff preview before deploying changes.

## Implementation Plan
1. [x] Expand GCP domain model & client - add UpdateService(), CreateService(), new fields
2. [x] Add edit/create message types
3. [x] Create CloudRunEditView with form, diff preview, save flow
4. [x] Wire into application (ViewType, navigation, rendering, breadcrumbs)
5. [x] Add trigger points ('e' in details, 'c' in list)
6. [x] Tests, lint, documentation

## Key Decisions
- Uses manual state machine (not CreateViewBase) because of diff preview state
- State flow: stateLoading → stateForm → stateDiff → stateSaving → complete/error
- Reuses existing diff.Viewer component for change preview
- Env vars edited as KEY=VALUE text area, secret refs preserved as read-only hint
- Both edit and create share the same view (isCreate flag)
