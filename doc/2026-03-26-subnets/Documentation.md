# Subnets List and Management

## Summary

Added standalone subnet management under VPC Network in the sidebar navigation. Subnets were previously only visible within the Network Details view's Subnets tab. They are now a first-class navigable resource with list, details, create, and delete support.

## Changes

### New Files

| File | Purpose |
|------|---------|
| `internal/ui/views/subnets.go` | Table-based list view for all subnets across networks/regions |
| `internal/ui/views/subnet_details.go` | Scrollable details view with secondary IP ranges, flow log config |
| `internal/ui/views/subnet_create.go` | Form-based creation view (CreateViewBase) |
| `internal/ui/views/subnet_messages.go` | Message types for subnet flows |
| `internal/ui/views/subnets_test.go` | List view tests |
| `internal/ui/views/subnet_details_test.go` | Details view tests |
| `internal/ui/views/subnet_create_test.go` | Create view tests |

### Modified Files

| File | Changes |
|------|---------|
| `internal/gcp/networks.go` | Added Network field to Subnet, SubnetDetails/FlowLogConfig/SecondaryRange/SubnetCreateConfig structs, ListAllSubnets/GetSubnetDetails/CreateSubnet/DeleteSubnet methods |
| `internal/gcp/networks_test.go` | Tests for new structs and methods |
| `internal/ui/app.go` | ViewType enum, App struct fields, getCurrentViewModel, updateViewSizes, message handlers |
| `internal/ui/app_render.go` | renderCurrentView cases, breadcrumbs |
| `internal/ui/app_navigation.go` | Navigation handlers, sidebar guards, clearAllViews |
| `internal/ui/components/sidebar/menu.go` | Subnets entry under Networking |
| `internal/ui/components/commandpalette/commands.go` | nav:subnets command |
| `internal/ui/views/network_details.go` | Subnet links now navigable |

## Features

- **List view**: Flat table of all subnets across all VPCs and regions. Columns: Name, Network, Region, CIDR Range, Purpose, Google Access, Flow Logs. Supports sorting, field-based filtering, action menu.
- **Details view**: Scrollable view with Basic Info, Configuration, Flow Logs Config (conditional), and Secondary IP Ranges sections. Network name is a navigable link to the network details view.
- **Create form**: Three sections — Basic Settings (name, description, network, region), IP Configuration (CIDR, purpose, stack type), Access & Logging (private Google access, flow logs). Network dropdown loaded asynchronously.
- **Delete**: Type-to-confirm from both list and details views.
- **Navigation**: Sidebar entry under VPC Network, command palette command, subnet links in Network Details are now clickable.

## Testing

```bash
make test   # All tests pass
make lint   # Zero issues
```
