# VPC Networks List View

## Task Description

Implement a VPC Networks list view that replaces the placeholder sidebar entry with a functional table view showing all VPC networks in the selected GCP project.

## Implementation Plan

1. [x] Create GCP networks data types and API client (`internal/gcp/networks.go`)
2. [x] Create networks list view (`internal/ui/views/networks.go`)
3. [x] Wire up app integration (app.go, app_render.go, app_navigation.go)
4. [x] Add unit tests (`internal/ui/views/networks_test.go`)
5. [x] Update documentation (key-bindings.md, CLAUDE.md)
6. [x] Build, lint, test verification

## Requirements

- Display VPC networks in a table with columns: Name, Subnet Mode, Routing, Subnets, Created
- Follow DisksView pattern for consistency
- Async client initialization and data loading with spinners
- Error handling with retry
- Filter/search support via table component
- Mouse click support via TableClickDelegate

## Decisions

- Reuse ComputeClient since VPC Networks are part of Compute Engine API
- No details view in this iteration (future work)
- NetworkSelectedMsg emitted on Enter but no navigation target yet
