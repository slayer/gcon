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

# Install linter (golangci-lint v2.1.6)
make install-lint

# Lint
make lint
```

## GCP Authentication

The app uses Application Default Credentials (ADC). Users authenticate via:

```bash
gcloud auth application-default login
```

Required scopes:
- `cloudresourcemanager.readonly` - List projects
- `compute` - Manage Compute Engine instances

## Code Style Guidelines

- Use descriptive variable names, avoid single letters except in loops
- Keep functions focused and small (<50 lines ideally)
- Add comments explaining WHY, not WHAT
- Handle errors explicitly, don't ignore them
- Use context.Context for cancellation in GCP API calls
- Export message types that need to cross package boundaries

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

## Bubble Tea Rendering Guidelines

### lipgloss Height() and Newline Counting
**Critical**: `lipgloss.Height(n)` renders n lines, which equals **n-1 newlines**.

When using `lipgloss.JoinHorizontal()` (e.g., sidebar + content), both sides must have the **same newline count** or the layout breaks and causes visual glitches like headers disappearing.

```go
// Sidebar uses lipgloss Height(n) which outputs n-1 newlines
container := styles.Container.Width(width).Height(s.height)
return container.Render(content)  // outputs height-1 newlines

// Content MUST match sidebar's newline count
func (v *View) renderLoading(msg string) string {
    content := fmt.Sprintf("\n  %s %s\n", v.spinner.View(), msg)
    // Match sidebar: height-1 newlines (NOT height!)
    targetNewlines := v.height - 1
    if targetNewlines < 10 {
        targetNewlines = 10
    }
    currentNewlines := strings.Count(content, "\n")
    for i := currentNewlines; i < targetNewlines; i++ {
        content += "\n"
    }
    return content
}
```

### Testing Render Heights
Always add tests to verify sidebar and content output the same newline count:

```go
func TestRenderingHeightConsistency(t *testing.T) {
    // Setup app with sidebar active
    sidebarView := app.sidebar.View()
    contentView := app.renderCurrentView()

    sidebarNewlines := strings.Count(sidebarView, "\n")
    contentNewlines := strings.Count(contentView, "\n")

    assert.Equal(t, sidebarNewlines, contentNewlines,
        "Sidebar and content must have same newline count")
}
```

### Avoid tea.ClearScreen
**Never use `tea.ClearScreen`** - it clears the entire terminal including app header/chrome, not just the content area. Instead, ensure consistent rendering heights.

### Hide Duplicate UI Elements
When using bubbles/list component, hide its built-in title with `l.SetShowTitle(false)` if the app header already shows context. This prevents duplicate titles.

### Unicode Symbol Width Issues
**Critical**: `lipgloss.Width()` miscounts certain Unicode symbols as 1-wide when terminals render them as 2-wide. Affected symbols include: ☁, ☰, ▶, ▸, ◀, ●

**Solutions**:
1. Use `SafeWidth(terminalWidth, content)` helper to reduce width by emoji count per line
2. Prefer 1-char Unicode symbols with lipgloss colors over emoji circles:
   - Instead of 🟢 🔴 🟡 → use ● with `lipgloss.Color("#34A853")` etc.
3. Centralize symbols in `internal/ui/symbols` package with ASCII fallback support (`--ascii` flag)

```go
// symbols/symbols.go - colored 1-char status indicators
var colorGreen = lipgloss.NewStyle().Foreground(lipgloss.Color("#34A853"))

func StatusRunning() string {
    if asciiMode {
        return colorGreen.Render("[OK]")
    }
    return colorGreen.Render("●")  // 1-char wide, colored
}
```

### Component Width Reporting
When components have borders, the `Width()` method must include border width in the reported value:

```go
func (s *Sidebar) Width() int {
    // Add 1 for the right border that the container style adds
    if s.collapsed {
        return CollapsedWidth + 1
    }
    return ExpandedWidth + 1
}
```

Layout calculations depend on accurate width reporting. If a component reports width=26 but renders as 27 (due to border), content will overflow by 1 character.

### lipgloss Style Testing
`lipgloss.Style.String()` returns an empty string - it doesn't serialize style properties. To test that styles are properly initialized, render content instead:

```go
// Wrong - String() returns empty
assert.NotEmpty(t, style.String())

// Correct - test by rendering
rendered := style.Render("test")
assert.NotEmpty(t, rendered)
```

### Component Width Caching
For components that recalculate layouts (like tables with flexible columns), cache the last width to avoid recalculation on every render:

```go
type Model struct {
    lastWidth       int
    columnsComputed bool
}

func (m *Model) SetSize(width, height int) {
    // Only recalculate if width changed
    if width != m.lastWidth || !m.columnsComputed {
        m.adjustColumnWidths(width)
        m.lastWidth = width
        m.columnsComputed = true
    }
}
```

## Key Bindings

### Global

| Key | Action |
|-----|--------|
| `q`, `Ctrl+C` | Quit |
| `?` | Toggle help |
| `Esc` | Go back |
| `j/k` or `↓/↑` | Navigate list |
| `r` | Refresh current view |
| `/` | Search/Filter |
| `Enter` | Select/Confirm |
| `:` or `Ctrl+K` | Open command palette |

### Instances View

| Key | Action |
|-----|--------|
| `s` | Start stopped instance |
| `x` | Stop running instance |
| `R` | Reset (hard reboot) |

### Instance Details - Observability Tab

| Key | Action |
|-----|--------|
| `1` | Set time range to 1 hour |
| `2` | Set time range to 6 hours |
| `3` | Set time range to 24 hours |
| `4` | Set time range to 7 days |
| `5` | Set time range to 30 days |
| `a` | Toggle auto-refresh (30s interval, on by default) |
| `r` | Manual refresh metrics |

### Images View

| Key | Action |
|-----|--------|
| `Enter` | View image details |
| `/` | Filter images |
| `r` | Refresh list |

### Objects View (GCS Browser)

| Key | Action |
|-----|--------|
| `Enter` | Open folder / View file details |
| `d` | Download file/folder |
| `u` | Upload files |
| `D` | Delete file/folder (with confirmation) |
| `.` | Open action menu |
| `n` | Next page |
| `p` | Previous page |

### Object Details View

| Key | Action |
|-----|--------|
| `v` | Preview text file (< 500KB) |
| `o` | Download and open with default app |
| `d` | Download to current directory |
| `D` | Delete (with confirmation) |
| `.` | Open action menu |
| `r` | Refresh metadata |
| `Tab` | Switch between tabs and content |
| `h/l` or `1-2` | Switch tabs (Details/Preview) |
| `Esc` | Back to objects list |


## _IMPORTANT NOTICES_

### _IMPORTANT NOTICE #1_ Workflow

- use current date in format `YYYY-mm-dd` for `{task_id}`, like `2025-12-11`
- Before starting any task, check if similar task is already in progress or completed to avoid duplication.
- For every new task, create a new branch named `{task_id}-<short_description>`
- On every task, create a `doc/{task_id}-{short description}/TODO.md` file. (see branch naming conventions below)
  - For subtasks, create a `doc/{task_id}-{short description}/TODO_<subtask_name>.md` file.
- Use this file to outline the task, including:
  - Task description
  - Implementation plan
  - Any specific requirements or constraints
- Break down the task into smaller, manageable steps.
- Add to it the task description, and plan the implementation.
- Use this TODO file to track your progress and document decisions.
- Periodically mark completed tasks as done in the TODO file.
- After completing the task, create/update file `doc/{task_id}-{short description}/Documentation.md` with:
  - Summary of changes made
  - Any relevant technical details
  - Instructions for testing or deployment if applicable
  - Mermaid diagrams if necessary to illustrate complex workflows or architectures.
- Create minimal but comprehensive tests to cover new features or bug fixes.
- Once the task is complete, create a Pull Request (PR) to merge your changes back

### _IMPORTANT NOTICE #3_ Code formatting

Use `go fmt` to format your code before committing.

### _IMPORTANT NOTICE #4_ Run tests frequently

When implementing any code edits - run corresponding tests frequently to catch breaking changes at earliest.
Always run full test suite before making decision that some step is done and moving over to the next step.
Run linter before committing code changes.

### _IMPORTANT NOTICE #5_ Branches and Commit messages

When working on a task, create a new branch named `{task_id}-<short_description>`. Description should be concise and reflect the task's purpose, and less than 32 characters long.
For example, `2025-12-11-project-selection`.

When committing code changes, use clear and descriptive commit messages. Follow the format:
```
{task_id}: <short description of changes>
```

### _IMPORTANT NOTICE #6_ Subagents

Spin up multiple subagents for each task to ensure parallel development. Each subagent should work on a specific aspect of the task, such as implementation, testing, or documentation. This will help speed up the development process and ensure that all aspects of the task are covered.

### Git and Merge Requests

- Use Git for version control.
- Project hosted on GitHub.
- Create Pull Requests for code reviews before merging changes to the main branch.
- Use descriptive titles and descriptions for Pull Requests to facilitate the review process.
- Use GitHub MCP if available for GitHub related tasks.

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
- [x] Command palette with fuzzy search
- [x] Recent items tracking
- [x] Sidebar navigation

## Planned Features

- [ ] Disk image deletion with confirmation
- [ ] Disk image creation from disks/snapshots
- [ ] Cloud Logging viewer with filters
- [ ] SSH to instance (via gcloud)
- [ ] Resource caching
- [ ] GKE cluster management
- [ ] Cloud Run services
- [ ] IAM management

