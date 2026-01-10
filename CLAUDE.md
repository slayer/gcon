# GCP TUI - Terminal UI for Google Cloud Platform

## Project Overview

A terminal-based user interface for managing Google Cloud Platform resources, built with Go and Bubble Tea.

## Tech Stack

- **Language**: Go 1.22+
- **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm-inspired architecture)
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **GCP SDK**: `cloud.google.com/go` + `google.golang.org/api`
- **Auth**: Application Default Credentials (ADC)

## Project Structure

```
.
├── cmd/
│   └── gcptui/           # Main entry point
│       └── main.go
├── internal/
│   ├── gcp/              # GCP API clients & abstractions
│   │   ├── client.go     # Base client with auth
│   │   ├── projects.go   # Project management
│   │   ├── compute.go    # Compute Engine
│   │   ├── storage.go    # Cloud Storage
│   │   └── logging.go    # Cloud Logging
│   ├── ui/
│   │   ├── app.go        # Main application model
│   │   ├── keys.go       # Key bindings
│   │   ├── styles.go     # Lip Gloss styles
│   │   ├── views/        # Screen components
│   │   │   ├── projects.go
│   │   │   ├── instances.go
│   │   │   ├── buckets.go
│   │   │   └── logs.go
│   │   └── components/   # Reusable UI widgets
│   │       ├── list.go
│   │       ├── table.go
│   │       ├── spinner.go
│   │       └── help.go
│   ├── config/           # Configuration management
│   │   └── config.go
│   └── cache/            # Local caching layer
│       └── cache.go
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Architecture Notes

### Bubble Tea Model Pattern

Every view implements the Bubble Tea model interface:

```go
type Model interface {
    Init() tea.Cmd
    Update(tea.Msg) (tea.Model, tea.Cmd)
    View() string
}
```

### Message Types

Define custom messages for async operations:

```go
// GCP API response messages
type ProjectsLoadedMsg struct { Projects []string }
type InstancesLoadedMsg struct { Instances []*compute.Instance }
type ErrorMsg struct { Err error }
```

### Async API Calls

Use tea.Cmd for non-blocking GCP API calls:

```go
func fetchProjects(client *gcp.Client) tea.Cmd {
    return func() tea.Msg {
        projects, err := client.ListProjects(context.Background())
        if err != nil {
            return ErrorMsg{Err: err}
        }
        return ProjectsLoadedMsg{Projects: projects}
    }
}
```

## Key Dependencies

```go
// go.mod essentials
require (
    github.com/charmbracelet/bubbletea v1.2.4
    github.com/charmbracelet/lipgloss v1.0.0
    github.com/charmbracelet/bubbles v0.20.0  // Pre-built components
    cloud.google.com/go/compute v1.x.x
    cloud.google.com/go/storage v1.x.x
    cloud.google.com/go/logging v1.x.x
    google.golang.org/api v0.x.x
)
```

## Development Commands

```bash
# Run the application
go run ./cmd/gcptui

# Build binary
go build -o gcptui ./cmd/gcptui

# Run tests
go test ./...

# Lint
golangci-lint run

# Update dependencies
go mod tidy
```

## GCP Authentication

The app uses Application Default Credentials (ADC). Users authenticate via:

```bash
gcloud auth application-default login
```

## Code Style Guidelines

- Use descriptive variable names, avoid single letters except in loops
- Keep functions focused and small (<50 lines ideally)
- Add comments explaining WHY, not WHAT
- Handle errors explicitly, don't ignore them
- Use context.Context for cancellation in GCP API calls

## UI/UX Guidelines

- Show loading spinners during API calls
- Cache frequently accessed data (projects list, etc.)
- Provide keyboard shortcuts for all actions
- Display errors in a non-blocking way
- Use consistent color scheme via Lip Gloss styles

## Key Bindings Convention

| Key | Action |
|-----|--------|
| `q`, `Ctrl+C` | Quit |
| `?` | Toggle help |
| `Enter` | Select/Confirm |
| `Esc` | Go back/Cancel |
| `j/k` or `↓/↑` | Navigate list |
| `r` | Refresh current view |
| `/` | Search/Filter |
| `Tab` | Switch panels |

## Initial MVP Features

1. [ ] Project selector
2. [ ] Compute Engine instances list (start/stop/SSH)
3. [ ] Cloud Storage buckets browser
4. [ ] Cloud Logging viewer with filters
5. [ ] Resource search across project

## Future Features

- GKE cluster management
- Cloud Run services
- Cloud Functions
- IAM management
- Cost overview
