# VPC Networks List View - Documentation

## Summary of Changes

Added a VPC Networks list view that replaces the placeholder sidebar entry with a fully functional table view displaying all VPC networks in the selected GCP project.

## Files Changed

### New Files
- `internal/gcp/networks.go` — Network data type and `ListNetworks` method on ComputeClient
- `internal/ui/views/networks.go` — Networks list view following DisksView pattern
- `internal/ui/views/networks_test.go` — Unit tests for view creation, loading state, and row conversion

### Modified Files
- `internal/ui/app.go` — Added `networksView` field, `selectedNetwork` field, `getCurrentViewModel()` case, `NetworkSelectedMsg` handler, `updateViewSizes()` context propagation
- `internal/ui/app_render.go` — Replaced placeholder with actual view rendering in `renderCurrentView()`
- `internal/ui/app_navigation.go` — Implemented sidebar navigation handler, added to `clearAllViews()`, added to `reloadCurrentView()` for project switching
- `.claude/rules/key-bindings.md` — Added Networks View key bindings section
- `CLAUDE.md` — Added VPC Networks to implemented features, added networking planned features

## Technical Details

### Architecture
- Reuses `ComputeClient` since VPC Networks are part of the Compute Engine API
- Follows the established async pattern: Init → spinner → client ready → load data → update table
- Embeds `TableClickDelegate` for mouse click support
- Implements `HasTextInputFocused()` for safe text filtering

### Table Columns
| Column | Source | Notes |
|--------|--------|-------|
| Name | `network.Name` | Network resource name |
| Subnet Mode | `network.AutoCreate` | "Auto" or "Custom" |
| Routing | `network.RoutingMode` | "REGIONAL" or "GLOBAL" |
| Subnets | `len(network.Subnetworks)` | Count of subnets |
| Created | `network.CreationTimestamp` | ISO timestamp from API |

### Key Bindings
- `Enter` — Select network (emits `NetworkSelectedMsg`, no details view yet)
- `/` — Filter networks by name, mode, or routing
- `r` — Refresh network list
- `Esc` — Go back

## Future Work
- VPC Network details view (subnets, peerings, firewall rules count)
- Firewall rules list and management
- Subnets list and management
