# Cloud SQL Instance Management

## Task Description
Add Cloud SQL instance management to gcon: listing, details (with Databases/Backups tabs), and lifecycle actions (start/stop/restart/delete).

## Implementation Steps

- [x] Step 1: GCP SQL Client (`internal/gcp/sql.go` + tests)
- [x] Step 2: SQL Messages (`internal/ui/views/sql_messages.go`)
- [x] Step 3: Sidebar category (`internal/ui/components/sidebar/menu.go`)
- [x] Step 4: SQL Instances List View
- [x] Step 5: SQL Instance Details View (3 tabs)
- [x] Step 6: App Integration (app.go, app_render.go, app_navigation.go)
- [x] Step 7: Documentation updates
- [x] Step 8: Testing & Linting
