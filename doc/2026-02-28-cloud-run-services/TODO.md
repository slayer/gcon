# Cloud Run Services Feature

## Task Description
Add Cloud Run service management to gcon with list, details (3 tabs), delete, and traffic split editing.

## Implementation Plan
1. [x] GCP Client — `internal/gcp/cloudrun.go` (types + API methods)
2. [x] GCP Client Tests — `internal/gcp/cloudrun_test.go`
3. [x] View Messages — `internal/ui/views/cloudrun_messages.go`
4. [x] List View — `internal/ui/views/cloudrun_services.go`
5. [x] Detail View — `internal/ui/views/cloudrun_service_details.go`
6. [x] View Tests — `cloudrun_services_test.go` + `cloudrun_service_details_test.go`
7. [x] App Integration — app.go, app_render.go, app_navigation.go, app_commands.go
8. [x] Sidebar + Command Palette — sidebar/menu.go, commandpalette/commands.go
9. [x] Documentation — CLAUDE.md, key-bindings.md
10. [x] Verify — make build && make test && make lint
