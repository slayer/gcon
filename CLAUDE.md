# gcon - Terminal UI for Google Cloud Platform

## Project Overview

A terminal-based user interface for managing Google Cloud Platform resources, built with Go and Bubble Tea.

## Tech Stack

- **Language**: Go 1.22+
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm-inspired architecture)
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **Charts**: [ntcharts](https://github.com/NimbleMarkets/ntcharts) (braille time series)
- **GCP SDK**: `cloud.google.com/go` + `google.golang.org/api`
- **Auth**: Application Default Credentials (ADC)
- **Testing**: Go's built-in testing package
  - Mocking with interfaces and test implementations
  - use `testify` for assertions
  - use table-driven tests if applicable
- **Linting**: golangci-lint v2.6.0+ with 32 enabled linters
  - Comprehensive error handling, security, and code quality checks
  - Bubble Tea-specific complexity allowances

## Project Structure

```
.
├── cmd/
│   └── gcon/              # Main entry point
│       └── main.go
├── internal/
│   ├── gcp/                 # GCP API clients & abstractions
│   │   ├── client.go        # Base client with auth (Cloud Resource Manager)
│   │   ├── projects.go      # Project listing
│   │   ├── compute.go       # Compute Engine (instances start/stop/reset)
│   │   └── sql.go           # Cloud SQL (instances, databases, backups)
│   ├── ui/
│   │   ├── app.go           # Main application model & view routing
│   │   ├── keys.go          # Global key bindings
│   │   ├── styles.go        # Lip Gloss styles (GCP color palette)
│   │   ├── messages.go      # Shared message types
│   │   ├── views/           # Screen components
│   │   │   ├── projects.go  # Project selector view
│   │   │   └── instances.go # Compute Engine instances view
│   │   └── components/      # Reusable UI widgets
│   │       ├── spinner.go   # Loading spinner
│   │       ├── statusbar.go # Status bar widget
│   │       └── metricchart/ # Time series charts (ntcharts wrapper)
│   ├── config/              # Configuration management (planned)
│   └── cache/               # Local caching layer (planned)
├── go.mod
├── go.sum
├── Makefile
└── CLAUDE.md
```

## Architecture Notes

### Bubble Tea Model Pattern

Views implement a simplified interface (not full tea.Model to allow parent control):

```go
type View interface {
    Init() tea.Cmd
    Update(tea.Msg) tea.Cmd  // Returns cmd, parent handles model
    View() string
    SetSize(width, height int)
}
```

### Message Flow

1. Views emit messages (e.g., `ProjectSelectedMsg`)
2. App's Update() catches cross-view messages and handles navigation
3. View-specific messages stay within the view

### Async API Calls

Use tea.Cmd for non-blocking GCP API calls:

```go
func (v *InstancesView) loadInstances() tea.Cmd {
    return func() tea.Msg {
        instances, err := v.computeClient.ListInstances(ctx, v.projectID)
        if err != nil {
            return instancesErrorMsg{err: err}
        }
        return instancesLoadedMsg{instances: instances}
    }
}
```

### Navigation Pattern

- App holds `currentView` enum and view instances
- `esc` key returns to previous view (handled in App.Update)
- Selecting items emits messages that trigger view switches

## Development Commands

```bash
# Run the application
make run

# Build binary
make build

# Run with race detector
make dev

# Build for all platforms
make build-all

# Run tests
make test

# Install linter (golangci-lint latest)
make install-lint

# Lint
make lint

# Lint with auto-fix
make lint-fix
```

## GCP Authentication

The app uses Application Default Credentials (ADC). Users authenticate via:

```bash
gcloud auth application-default login
```

Required scopes:
- `cloudresourcemanager.readonly` - List projects
- `compute` - Manage Compute Engine instances
- `devstorage.full_control` - Manage Cloud Storage buckets and objects
- `sqladmin` - Manage Cloud SQL instances
- `cloud-platform` - IAM service accounts, keys, policies, and custom roles

## Code Style Guidelines

- Use descriptive variable names, avoid single letters except in loops
- Keep functions focused and small (<50 lines ideally)
- Add comments explaining WHY, not WHAT
- Handle errors explicitly, don't ignore them
- Use context.Context for cancellation in GCP API calls
- Export message types that need to cross package boundaries
- Use modern Go features
  - generics, error wrapping where appropriate
  - `any` instead of `interface{}` when type is truly generic

## Documentation Guidelines

### Internal Package Documentation

Internal packages should include README.md files when they:
- Have non-obvious behavior or performance characteristics
- Provide reusable functionality across multiple packages
- Have complex APIs with multiple functions
- Require setup/teardown or lifecycle management

Example: `internal/debug/README.md` documents the debug logging API, performance warnings, and usage patterns.

README.md files in internal packages should include:
- Brief overview of the package's purpose
- API reference with code examples
- Usage instructions and best practices
- Performance or security considerations (if applicable)
- Testing guidance

## UI/UX Guidelines

- Show loading spinners during API calls
- Provide keyboard shortcuts for all actions
- Display errors inline with retry option
- Use consistent color scheme via Lip Gloss styles (GCP colors)
- Status indicators: ● green (running), ● red (stopped), ○ yellow (transitioning)

## Feature Development Guidelines

On developing or updating a new feature keep in mind the following guidelines:
- Ensure what any asynchronous operations (like reload, etc) should be displayed in footer (status bar) with spinner
- Add new key bindings to global key bindings list in `internal/ui/keys.go` and document them in README.md, navigation.md, etc.
- For view related actions - add them to context menu (ActionMenu) if applicable.
- Ensure that breadcrumbs are updated properly when navigating between views.

## Implemented Features

- [x] Project selector with search/filter
- [x] Compute Engine instances list
- [x] Compute Engine instance details view
- [x] Compute Engine instance observability tab with real-time metrics
  - CPU utilization with braille time series chart
  - Memory usage with braille time series chart (requires Ops Agent)
  - Network traffic statistics
  - Disk I/O metrics
  - Instance health and uptime
  - Automated recommendations based on metrics
  - Recent logs (warnings/errors)
  - Time range selection (1h/6h/24h/7d/30d)
  - Auto-refresh capability
- [x] Compute Engine persistent disks list
- [x] Compute Engine disk details view
- [x] Compute Engine disk images list
- [x] Compute Engine image details view
- [x] Instance actions (start/stop/reset/suspend/resume/delete)
- [x] Create new VM instances with form-based UI
  - Zone selection with dynamic machine type loading (cached per zone)
  - Curated boot disk images (Debian, Ubuntu, CentOS, Rocky, RHEL, Windows, COS)
  - Disk type selection (pd-balanced/pd-standard/pd-ssd)
  - Network and subnetwork selection (dynamically loaded)
  - External IP configuration (ephemeral/none)
  - Custom machine type support
- [x] Edit existing VM instance configuration
  - Machine type changes (with stopped-instance warning)
  - Boot disk resize (expand only)
  - Diff preview before applying changes
  - Sequential API calls with partial failure reporting
- [x] View navigation with breadcrumbs
- [x] Loading states with spinners
- [x] Error handling with retry
- [x] Cloud Storage buckets browser
- [x] Cloud Storage objects browser with upload/download
- [x] Cloud Storage bucket creation with forms UI
  - Location type selection (region/dual-region/multi-region)
  - Storage class selection (STANDARD/NEARLINE/COLDLINE/ARCHIVE)
  - Access control settings (uniform/fine-grained)
  - Data protection options (versioning, retention, soft delete)
  - Labels and CMEK encryption support
- [x] Cloud Storage bucket usage analysis
  - Cloud Monitoring metrics (total bytes, object count) shown inline in buckets list
  - On-demand deep scan with breakdowns by storage class, top-level prefix, and file extension
  - Live progress in footer task with `Ctrl+X` to cancel
  - Folder-scoped deep scan from objects browser (`C` key) with inline stats
  - Bucket details view (`i` key) with Details and Usage tabs
- [x] Command palette with fuzzy search
- [x] Recent items tracking
- [x] Sidebar navigation
- [x] Project selector modal for quick project switching
  - Trigger via command palette ("Switch Project")
  - Real-time filtering by project name or ID
  - Async project loading with spinner
  - Shows automatically on startup if no default project
  - Reloads all views when switching projects
- [x] VPC Networks list view
- [x] VPC Network details view with tabs (Details/Subnets)
- [x] Subnets list and management
  - List all subnets across networks and regions with table view
  - Subnet details view with secondary IP ranges and flow log config
  - Create subnet with network/region/CIDR/purpose/stack type selection
  - Delete subnet with type-to-confirm
  - Navigate from Network Details subnets tab to subnet details
  - Sidebar entry under Networking, command palette integration
- [x] Firewall rules list and details view
  - List firewall rules with direction, priority, action, protocols, status
  - View rule details with allowed/denied entries, source/destination ranges
  - Enable/disable firewall rules
  - Delete firewall rules with confirmation
  - Navigate to associated VPC network
- [x] VPC Routes management
  - List all routes with network, destination, priority, next hop, type (Static/Subnet/Peering/System)
  - Route details view with routing info, network link, and warnings
  - Create route with next hop type selection (Gateway/Instance/IP/VPN Tunnel/Interconnect/ILB)
  - Delete static routes with type-to-confirm (system/subnet/peering routes are read-only)
  - Routes tab in Network Details view
  - Sidebar entry under Networking, command palette integration
- [x] Cloud SQL instance management
  - List SQL instances with version, state, region, tier, IP
  - Instance details view with 3 tabs (Details/Databases/Backups)
  - Lifecycle actions (start/stop/restart/delete with confirmation)
  - List databases per instance
  - List backup runs and create on-demand backups
- [x] Table enhancements
  - Column sorting via popup sort menu (`S` key) with numeric-aware comparison
  - Field-based filtering (`field:value` syntax, e.g. `status:running zone:us`)
  - Infinite scroll for Objects view (replaces page navigation)
- [x] IAM & Admin management
  - Service Accounts list with status indicators (active/disabled)
  - Service Account details with 2 tabs (Details/Keys)
  - Create service accounts with form validation
  - Delete service accounts with type-to-confirm
  - Enable/disable service accounts
  - Service account key management (create/delete)
  - Key JSON download on creation (saved to current directory)
  - IAM Policy bindings view with 2 tabs (By Member/By Role)
  - Table-based IAM policy view with sorting, filtering, row selection
  - Add/remove members to/from role bindings (with input validation)
  - Detail overlay for viewing member's roles or role's members
  - Read-modify-write with etag conflict retry for safe policy updates
  - Custom Roles list and details view (read-only)
  - Custom Role details with 2 tabs (Details/Permissions)
- [x] Cloud Run services
  - List services across all regions with status, URL, latest revision
  - Service details view with 4 tabs (Details/Revisions/YAML/Observability)
  - Traffic split editing dialog (validates percentages sum to 100)
  - Delete service with type-to-confirm
  - Container config, scaling, environment variables, labels display
  - Revision list with traffic percentages and status indicators
  - Edit existing service configuration with diff preview before deploy
  - Create new services with full form (container, scaling, networking, security)
  - Form-based editor with 7 sections and validation
  - Observability tab with request metrics, resource metrics, and filterable logs
    - Request count with braille time series chart
    - Request latency (p50/p95/p99) with multi-series overlay chart
    - Error rate (4xx/5xx) with multi-series overlay chart
    - CPU utilization with braille time series chart
    - Billable instance time with braille time series chart
    - Instance count with braille time series chart
    - Filterable log viewer (INFO/WARNING/ERROR severity toggles)
    - Time range selection (1h/6h/24h/7d/30d)
    - Auto-refresh capability (30s interval)
- [x] Cloud Logging Explorer
  - LQL query input with filter bar
  - Quick filters (Resources, Log Names, Severities) with lazy-loaded options and search
  - Tab cycling between entries, filters, query input, and time range
  - Sparkline histogram for log density over time
  - Expandable log entries with severity color coding
  - Logfmt and protobuf key:value syntax colorization (toggle with `c`)
  - Field-level cursor with filter-by-field (Enter on expanded field)
  - Infinite scroll pagination (200 entries per page)
  - Tail mode (live streaming, 15s polling)
  - Time range selection (1h/6h/24h/7d/30d)
  - Line wrapping toggle (`w` key)
  - Open in $PAGER with `p` key (respects color toggle)
  - Export to TXT/CSV/JSONL via action menu
  - ANSI-aware truncation and wrapping (preserves existing log colors)

## Planned Features
- [x] Subnets list and management
- [x] Disk image deletion with confirmation
- [x] Disk image creation from disks/snapshots
- [ ] SSH to instance (via gcloud)
- [ ] Resource caching
- [ ] GKE cluster management
