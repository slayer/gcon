# gcon - Terminal UI for Google Cloud Platform

## Project Overview

A terminal-based user interface for managing Google Cloud Platform resources, built with Go and Bubble Tea.

## Tech Stack

- **Language**: Go 1.22+
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm-inspired architecture)
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss)
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
│   │   └── compute.go       # Compute Engine (instances start/stop/reset)
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
│   │       └── statusbar.go # Status bar widget
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
  - CPU utilization with sparkline trends
  - Memory usage with sparkline trends (requires Ops Agent)
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
- [x] Instance actions (start/stop/reset)
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
- [x] Firewall rules list and details view
  - List firewall rules with direction, priority, action, protocols, status
  - View rule details with allowed/denied entries, source/destination ranges
  - Enable/disable firewall rules
  - Delete firewall rules with confirmation
  - Navigate to associated VPC network

## Planned Features
- [ ] Subnets list and management
- [ ] Disk image deletion with confirmation
- [ ] Disk image creation from disks/snapshots
- [ ] Cloud Logging viewer with filters
- [ ] SSH to instance (via gcloud)
- [ ] Resource caching
- [ ] GKE cluster management
- [ ] Cloud Run services
- [ ] IAM management
