# gcon - Terminal UI for Google Cloud Platform

## Project Overview

A terminal-based user interface for managing Google Cloud Platform resources, built with Go and Bubble Tea.

## Tech Stack

- **Language**: Go 1.26+ (toolchain pinned via `mise.toml`)
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm-inspired architecture)
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **Charts**: [ntcharts](https://github.com/NimbleMarkets/ntcharts) (braille time series)
- **GCP SDK**: `cloud.google.com/go` + `google.golang.org/api`
- **Auth**: Application Default Credentials (ADC)
- **Testing**: Go's built-in testing package
  - Mocking with interfaces and test implementations
  - use `testify` for assertions
  - use table-driven tests if applicable
- **Linting**: golangci-lint v2.12+ with 32 enabled linters
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
  - Parent-folder navigation: `..` row at the top, `←` / Enter on `..` to go up, `→` to drill into a folder
  - Multi-select with `Space` (toggle) / `*` (select-all on visible rows)
  - Bulk **delete** and **download** — folders auto-expanded to recursive members
  - Bulk **change storage class** — picker (STANDARD/NEARLINE/COLDLINE/ARCHIVE), server-side rewrite per object
  - Status bar shows `[N selected]` while a bulk selection is active; Esc clears the selection before navigating back
- [x] Cloud Storage bucket creation with forms UI
  - Location type selection (region/dual-region/multi-region)
  - Storage class selection (STANDARD/NEARLINE/COLDLINE/ARCHIVE)
  - Access control settings (uniform/fine-grained)
  - Data protection options (versioning, retention, soft delete)
  - Labels and CMEK encryption support
- [x] Cloud Storage bucket usage analysis
  - Cloud Monitoring metrics (total bytes, object count) shown inline in buckets list
  - On-demand deep scan with breakdowns by storage class, top-level prefix, and file extension
  - Footer spinner during scan; results delivered when complete (`Ctrl+X` to cancel)
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
- [x] Load Balancers (Phase 1)
  - List forwarding rules across global + all regions, with type derivation
    (HTTPS external / internal, HTTP, TCP/SSL proxy, Network LB proxy/passthrough/legacy)
  - Details view with three tabs: Overview, Routing (URL map host/path → backend),
    Backends (instance groups / NEGs, health checks, balancing mode)
  - Delete with dependency cascade: walks proxy → URL map → backend services →
    health checks, skipping shared resources; type-to-confirm dialog lists every
    resource that will be deleted and every shared one that will be kept
- [x] Load Balancers (Phase 2) — live backend health + Observability tab
  - Backend health: `backendServices.getHealth` per group, inline `● N/M healthy`
    badges on the Backends tab, expand with `Tab`/`Enter` for per-instance
    HEALTHY / UNHEALTHY / DRAINING state and IP:port.
  - Serverless NEGs (Cloud Run / Cloud Functions / App Engine) are auto-detected
    via the NEG's `networkEndpointType` and skipped with a labeled placeholder.
  - Observability tab (HTTP / HTTPS / internal HTTPS only): request count,
    request latency (p50/p95/p99), error rate (4xx/5xx as a percentage of
    total requests), backend latency, and throughput (bytes in / out).
  - Time-range selector (1h/6h/24h/7d/30d), auto-refresh on a 30 s tick,
    manual `r` refresh.
  - Network LBs (passthrough / proxy / legacy) render an explicit
    placeholder on the Observability tab — `l3/*` metric family is on
    the roadmap.
- [x] GKE cluster management (Phase 1 + 2a + 2b + 2c + 2d)
  - Clusters list across all locations (zonal + regional, Autopilot + Standard)
  - Cluster details with Overview, Node Pools, Nodes, Observability, and Logs tabs
  - Type-to-confirm cluster delete with explicit warning about non-auto-deleted
    resources (dynamic-provisioned PVs, LB Services, Cloud DNS)
  - Fire-and-forget delete: API call returns immediately; refresh shows status
  - Nodes tab: flat list across all pools derived from MIG enumeration; Enter
    opens Compute Engine instance details
  - Observability tab: five cluster-scope charts (CPU%, Memory%, Node count,
    Pod count, Network rx/tx) with time range selector and 30s auto-refresh
  - Logs tab: filterable embedded log viewer with severity toggles
    (INFO/WARNING/ERROR) and resource-type dropdown (cluster/node/pod/container)
  - Phase 2b mutations:
    - Create node pool with form (machine type, autoscaling, lifecycle settings)
    - Delete pool with type-to-confirm
    - Resize pool (manual count or autoscale min/max in one dialog)
    - Upgrade control plane and individual node pools (version picker)
    - Long-running operations tracked in footer with 5 s polling; refresh on DONE
    - Autopilot clusters hide pool actions; release-channel clusters hide master upgrade
  - Phase 2c edit flows (`e` key):
    - Cluster edit (Overview tab, both Standard + Autopilot): logging/monitoring services + daily maintenance window with diff preview before deploy
    - Node pool edit (Node Pools tab, Standard only): auto-upgrade/auto-repair toggles + upgrade strategy/max-surge/max-unavailable
    - Multi-endpoint edits (e.g. services + maintenance) run sequentially under one `gke-op:` footer task; status updates as each step lands
    - Unknown upgrade strategies (legacy `SHORT_LIVED`, etc.) surface as a placeholder without flagging a false-positive diff
  - Phase 2d editor completion (`l` / `t` keys within the edit forms):
    - Cluster resource labels editor (GCP-label rules) via `labeledit` overlay
    - Pool k8s labels editor (DNS subdomain prefixes, uppercase, dots) via `labeledit` with pluggable validators
    - Pool taints editor (key/value/effect rows) via new `taintedit` component
    - Recurring maintenance windows: days-of-week multi-select + start time + duration → builds `FREQ=WEEKLY;BYDAY=...` RRULE; round-trip preserves the form's defaults
    - Diff preview surfaces added/removed/changed entries with green/red/yellow colour coding

## Planned Features
- [x] Subnets list and management
- [x] Disk image deletion with confirmation
- [x] Disk image creation from disks/snapshots
- [x] SSH to running instance
  - `t` key opens an options dialog (method, user, IAP tunnel, internal IP, port forward)
  - Hands off via `gcloud compute ssh` by default; falls back to plain `ssh` when gcloud is absent
  - Returns to the TUI on exit; if the binary cannot launch, the error is shown inline (stderr from a started session is printed to the terminal during the session itself, not captured afterward)
- [ ] Resource caching
- [ ] GKE Phase 3: cluster create wizard
