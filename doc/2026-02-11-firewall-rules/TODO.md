# Firewall Rules Feature

## Task Description
Implement full Firewall Rules list and details views for GCP VPC firewall rules.

## Implementation Plan
1. [x] GCP client (`internal/gcp/firewalls.go` + tests)
2. [x] Message types (`internal/ui/views/firewall_messages.go`)
3. [x] List view (`internal/ui/views/firewalls.go`)
4. [x] Details view (`internal/ui/views/firewall_details.go`)
5. [x] App integration (app.go, app_render.go, app_navigation.go, app_footer.go)
6. [x] Tests for views
7. [x] Documentation updates (CLAUDE.md, README.md, key-bindings.md)
8. [x] Full test suite + lint pass

## Scope
- List view: Table with filter, refresh, action menu, delete, enable/disable
- Details view: Tabbed view (Details/Rules) with action menu, network link navigation
- GCP client: List, Get, Delete, Patch (enable/disable) operations
- No create/edit forms in this iteration
