# VPC Network Details View

## Summary of Changes

Added a Network Details page accessible by pressing Enter on a network in the VPC Networks list view. The details page has two tabs: **Details** (network configuration) and **Subnets** (subnet table with navigable links).

## Technical Details

### GCP Client Layer (`internal/gcp/networks.go`)
- Added `NetworkDetails`, `NetworkPeering`, and `Subnet` structs
- Added `GetNetworkDetails()` - calls `Networks.Get()` API
- Added `ListSubnetsByNetwork()` - uses `Subnetworks.AggregatedList()` with network filter, sorted by region+name
- Added `extractRegionFromURL()` and `extractNameFromURL()` helpers for parsing GCP resource URLs

### Networks List View (`internal/ui/views/networks.go`)
- Added Enter key binding and `NetworkSelectedMsg` export
- Added `findNetworkByID()` helper
- Updated help text to include "enter: details"

### Network Details View (`internal/ui/views/network_details.go`)
- Two tabs: Details and Subnets (following `instance_details.go` pattern)
- Focus manager with 3 regions: tabs, links (disabled until subnets load), viewport
- Links component for navigable subnet rows in Subnets tab
- Parallel loading of details and subnets on Init
- Action menu with Refresh action
- Mouse click region support via `Clickable` interface

### App Integration
- `app.go`: Added `ViewNetworkDetails` enum, `networkDetailsView` field, `selectedNetwork` field, message routing
- `app_render.go`: Added render case and breadcrumbs ("VPC Network > {network name}")
- `app_navigation.go`: Added `handleNetworkSelected()`, sidebar/clear/reload/back navigation handlers

### Files Changed
| File | Action |
|------|--------|
| `internal/gcp/networks.go` | Modified - added detail structs and methods |
| `internal/gcp/networks_test.go` | Created - tests for conversion functions |
| `internal/ui/views/networks.go` | Modified - Enter key and NetworkSelectedMsg |
| `internal/ui/views/networks_test.go` | Modified - tests for findNetworkByID, help text |
| `internal/ui/views/network_details.go` | Created - detail view with tabs/focus/links |
| `internal/ui/views/network_details_test.go` | Created - tests for helpers and constructor |
| `internal/ui/app.go` | Modified - ViewType, fields, message routing |
| `internal/ui/app_render.go` | Modified - render case and breadcrumbs |
| `internal/ui/app_navigation.go` | Modified - navigation handlers |
| `CLAUDE.md` | Modified - feature list updated |
| `.claude/rules/key-bindings.md` | Modified - key bindings updated |

## Testing
- `make test` - all 28 packages pass
- `make lint` - 0 issues
