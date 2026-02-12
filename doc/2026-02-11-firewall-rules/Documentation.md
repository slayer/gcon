# Firewall Rules Feature

## Summary

Added firewall rules list and details views to the VPC Network section, enabling users to browse, inspect, enable/disable, and delete GCP VPC firewall rules directly from the terminal UI.

## Changes

### New Files

| File | Purpose |
|------|---------|
| `internal/gcp/firewalls.go` | GCP client types and methods for firewall CRUD operations |
| `internal/gcp/firewalls_test.go` | Tests for GCP client helpers and conversion functions |
| `internal/ui/views/firewall_messages.go` | Cross-view message types for firewall operations |
| `internal/ui/views/firewalls.go` | List view with table, filter, action menu, delete, toggle |
| `internal/ui/views/firewalls_test.go` | Tests for list view (constructor, rendering, row conversion) |
| `internal/ui/views/firewall_details.go` | Details view with tabs (Details/Rules), action menu, network link |
| `internal/ui/views/firewall_details_test.go` | Tests for details view (constructor, tabs, accessors) |

### Modified Files

| File | Change |
|------|--------|
| `internal/ui/app.go` | Added `ViewFirewallDetails` enum, view fields, message handlers |
| `internal/ui/app_render.go` | Added rendering for firewall views and breadcrumbs |
| `internal/ui/app_navigation.go` | Added navigation handlers, sidebar init, cleanup |
| `internal/ui/app_footer.go` | Added `ViewFirewallDetails` to "esc back" group |
| `CLAUDE.md` | Moved firewall from Planned to Implemented |
| `README.md` | Added firewall features and key bindings |
| `.claude/rules/key-bindings.md` | Added Firewalls View and Firewall Details View sections |

## Key Bindings

| Key | Action |
|-----|--------|
| `t` | Enable/disable firewall rule |
| `D` | Delete firewall rule (with type-to-confirm) |
| `.` | Open action menu |
| `/` | Filter rules |
| `r` | Refresh |
| `Enter` | View details / navigate to network |
| `Tab` | Switch focus between tabs, links, content |
| `h/l` or `1/2` | Switch tabs (Details/Rules) |

## Technical Notes

- **ForceSendFields**: `SetFirewallRuleDisabled` uses `ForceSendFields: []string{"Disabled"}` to ensure GCP accepts `false` value (normally omitted as zero-value bool)
- **Paginated listing**: Uses `Firewalls.List().Pages()` for full pagination, sorted by priority then name
- **Network navigation**: Details view links to the associated VPC network via `NetworkSelectedMsg`, reusing existing cross-view navigation
- **Pattern followed**: Networks list/details views (table with `TableClickDelegate`, tabbed details with `focus.Manager`)
