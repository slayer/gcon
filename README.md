# gcon

<p align="center">
  <strong>A modern terminal UI for managing Google Cloud Platform resources</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/go-1.22+-blue" alt="Go Version">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="License"></a>
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows-lightgrey" alt="Platform">
</p>

---

## Overview

**gcon** is a powerful, keyboard-driven terminal user interface (TUI) for managing Google Cloud Platform resources. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and Go, it provides a fast, intuitive alternative to the GCP Console and `gcloud` CLI for common cloud operations.

**Why gcon?**

- 🚀 **Fast & Efficient** - Navigate your GCP resources with keyboard shortcuts, no mouse needed
- 💻 **Terminal Native** - Works seamlessly in any terminal, perfect for SSH sessions and remote work
- 🎨 **Beautiful UI** - Modern terminal interface with GCP color scheme, spinners, and status indicators
- ⚡ **Async Operations** - Non-blocking API calls with real-time loading indicators
- 🔍 **Fuzzy Search** - Quickly find projects, instances, and resources with built-in filtering
- 📊 **Resource Monitoring** - Real-time metrics, logs, and observability for your instances

## Features

### ✅ Currently Implemented

#### Project Management
- 🔄 **Project Selector** - Browse all accessible GCP projects with fuzzy search and filtering
- 🔀 **Quick Project Switching** - Switch between projects via command palette without losing context
- 📋 **Project Metadata** - View and edit project metadata and labels
- 🎯 **Default Project** - Automatic detection from gcloud config or environment variables

#### Compute Engine
- 📦 **VM Instance Management**
  - List all instances with status indicators (running, stopped, transitioning)
  - View detailed instance information (machine type, zones, IPs, tags, labels)
  - Start, stop, reset, suspend, and resume instances
  - Delete instances with confirmation dialogs
  - Create new instances with guided forms
  - Real-time status updates with visual indicators

- 📈 **Instance Observability**
  - CPU utilization with sparkline trends
  - Memory usage monitoring (requires Ops Agent)
  - Network traffic statistics (ingress/egress)
  - Disk I/O metrics
  - Instance health and uptime tracking
  - Automated performance recommendations
  - Recent error and warning logs
  - Multiple time ranges (1h, 6h, 24h, 7d, 30d)
  - Auto-refresh capability

- 💾 **Persistent Disk Management**
  - List all disks with details (size, type, status, attachments)
  - View disk details and usage information
  - Create new disks from scratch or from snapshots
  - Delete disks with safety confirmations
  - Link navigation to attached instances

- 📸 **Snapshot Management**
  - List all disk snapshots
  - View snapshot details and source disks
  - Create snapshots from existing disks
  - Navigate to source disks from snapshots

- 🖼️ **Image Management**
  - List all disk images (custom and public)
  - View image details and properties
  - Create images from disks or snapshots
  - Support for various image families

- 🏷️ **Metadata & Labels**
  - View and edit instance metadata
  - Manage labels on instances and disks
  - Bulk label operations

#### Cloud Storage
- 🪣 **Bucket Management**
  - List all Cloud Storage buckets
  - View bucket details (location, storage class, access control)
  - Create new buckets with comprehensive options:
    - Location type selection (region/dual-region/multi-region)
    - Storage class selection (STANDARD/NEARLINE/COLDLINE/ARCHIVE)
    - Access control settings (uniform/fine-grained)
    - Data protection (versioning, retention, soft delete)
    - Labels and CMEK encryption support

- 📁 **Object Browser**
  - Navigate bucket contents with folder structure
  - Upload files and folders
  - Download individual files or entire folders
  - Delete objects with confirmation
  - View object details and metadata
  - Pagination for large buckets

#### User Interface & Navigation
- 🎯 **Command Palette** - Quick access to all actions with fuzzy search (`:` or `Ctrl+K`)
- 📂 **Sidebar Navigation** - Persistent sidebar with resource categories
- 🍞 **Breadcrumb Navigation** - Always know where you are in the resource hierarchy
- 📜 **Recent Items** - Quick access to recently viewed resources
- ⌨️ **Keyboard Shortcuts** - Vim-style navigation throughout the interface
- 🔄 **Context Menus** - Action menus for resource-specific operations
- ⚡ **Loading States** - Spinners and progress indicators for all async operations
- ❌ **Error Handling** - Inline error display with retry options
- 📊 **Status Bar** - Real-time status updates and operation feedback

### 🚧 Planned Features

The following features are planned for future releases:

- [ ] **Cloud Logging** - Integrated log viewer with advanced filters and search
- [ ] **SSH Integration** - Direct SSH access to instances via gcloud
- [ ] **Resource Caching** - Local caching layer for faster repeated queries
- [ ] **Google Kubernetes Engine (GKE)**
  - Cluster listing and details
  - Node pool management
  - Workload viewing
- [ ] **Cloud Run**
  - Service management
  - Revision history
  - Traffic splitting
- [ ] **IAM Management**
  - View IAM policies
  - Manage service accounts
  - Role assignments
- [ ] **Cloud SQL** - Database instance management
- [ ] **Cloud Functions** - Function deployment and monitoring
- [ ] **VPC Networks** - Network topology and firewall rules
- [ ] **Load Balancers** - Load balancer configuration and health
- [ ] **Cost Explorer** - Resource cost analysis and budgets

## Installation

### From Source

```bash
go install github.com/slayer/gcon/cmd/gcon@latest
```

### From Releases

Download the latest binary from [Releases](https://github.com/slayer/gcon/releases).

## Authentication

gcon uses Google Cloud's Application Default Credentials (ADC). Choose one of these methods:

### Option 1: User Credentials (requires gcloud CLI)

```bash
gcloud auth application-default login
```

### Option 2: Service Account Key

```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

### Option 3: Workload Identity (GCP environments)

When running on GKE, Cloud Run, or GCE, authentication is automatic via the attached service account.

## Configuration

gcon respects standard GCP SDK environment variables:

| Variable | Description |
|----------|-------------|
| `CLOUDSDK_CORE_PROJECT` | Default project |
| `CLOUDSDK_COMPUTE_ZONE` | Default zone |
| `CLOUDSDK_COMPUTE_REGION` | Default region |
| `CLOUDSDK_CORE_ACCOUNT` | Default account |
| `CLOUDSDK_ACTIVE_CONFIG_NAME` | gcloud config to use |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to service account JSON |
| `NO_COLOR` | Disable colored output |
| `HTTP_PROXY` / `HTTPS_PROXY` | Proxy settings |

If gcloud CLI is installed, gcon also reads defaults from `~/.config/gcloud/`.

## Quick Start

1. **Install gcon** (see [Installation](#installation) section below)
2. **Authenticate with GCP**:
   ```bash
   gcloud auth application-default login
   ```
3. **Launch gcon**:
   ```bash
   gcon
   ```
4. **Select a project** - On first launch, you'll see the project selector
5. **Navigate resources** - Use arrow keys or vim bindings to navigate

## Usage

### Basic Navigation

Once launched, gcon presents an intuitive interface with:
- **Sidebar** (left) - Quick access to resource categories
- **Main View** (center) - Current resource list or details
- **Status Bar** (bottom) - Current operation status and key hints
- **Breadcrumbs** (top) - Your current location in the resource hierarchy

### Key Bindings

#### Global Shortcuts

| Key | Action | Description |
|-----|--------|-------------|
| `:` / `Ctrl+K` | Command Palette | Quick access to all commands with fuzzy search |
| `Ctrl+C` or `q` | Quit | Exit the application |
| `Esc` | Go Back | Return to previous view |
| `?` | Help | Show context-sensitive help |
| `r` | Refresh | Reload current view |
| `.` | Action Menu | Open context-sensitive action menu |
| `[` | Focus Sidebar | Move focus to sidebar |
| `]` | Focus Content | Move focus to main content area |
| `{` | Toggle Sidebar | Show or hide the sidebar |

#### Navigation

| Key | Action | Description |
|-----|--------|-------------|
| `j` or `↓` | Move Down | Move cursor down in lists |
| `k` or `↑` | Move Up | Move cursor up in lists |
| `h` or `←` | Move Left | Move left in horizontal navigation |
| `l` or `→` | Move Right | Move right in horizontal navigation |
| `g` | Go to Top | Jump to first item |
| `G` | Go to Bottom | Jump to last item |
| `Ctrl+D` | Page Down | Scroll down one page |
| `Ctrl+U` | Page Up | Scroll up one page |
| `Enter` | Select | Select current item or confirm action |
| `/` | Search/Filter | Filter current list |
| `Tab` | Next Section | Move to next section (in details views) |
| `Shift+Tab` | Previous Section | Move to previous section |

#### Compute Engine - Instances

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | View Details | Show detailed instance information |
| `s` | Start | Start a stopped instance |
| `x` | Stop | Stop a running instance |
| `r` | Reset | Reset (reboot) an instance |
| `p` | Suspend | Suspend a running instance |
| `R` | Resume | Resume a suspended instance |
| `D` | Delete | Delete instance (with confirmation) |
| `c` | Create | Create a new instance |
| `l` | Edit Labels | Open label editor |
| `m` | Metadata | View/edit instance metadata |
| `o` | Observability | View metrics and logs |

#### Compute Engine - Disks

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | View Details | Show detailed disk information |
| `c` | Create Disk | Create a new persistent disk |
| `D` | Delete | Delete disk (with confirmation) |
| `s` | Create Snapshot | Create a snapshot from this disk |
| `l` | Edit Labels | Manage disk labels |

#### Compute Engine - Snapshots

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | View Details | Show snapshot details |
| `c` | Create Snapshot | Create a new snapshot |
| `D` | Delete | Delete snapshot (with confirmation) |
| `d` | View Source Disk | Navigate to the source disk |

#### Compute Engine - Images

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | View Details | Show image details |
| `c` | Create Image | Create a new image |
| `D` | Delete | Delete custom image (with confirmation) |

#### Cloud Storage - Buckets

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | Browse Contents | Open bucket and view objects |
| `c` | Create Bucket | Create a new bucket with guided form |
| `D` | Delete | Delete bucket (must be empty) |
| `/` | Filter | Filter buckets by name |
| `r` | Refresh | Reload bucket list |

#### Cloud Storage - Objects

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | Open/View | Open folder or view file details |
| `u` | Upload | Upload files to current folder |
| `d` | Download | Download selected file or folder |
| `D` | Delete | Delete object (with confirmation) |
| `n` | Next Page | Navigate to next page |
| `p` | Previous Page | Navigate to previous page |
| `Backspace` | Go Up | Go to parent folder |

#### Label & Metadata Editors

| Key | Action | Description |
|-----|--------|-------------|
| `a` | Add | Add new label or metadata entry |
| `e` or `Enter` | Edit | Edit selected entry |
| `d` or `x` | Delete | Delete selected entry |
| `Ctrl+S` | Save | Save all changes |
| `Esc` | Cancel | Cancel without saving |
| `Tab` | Next Field | Move to next input field |
| `Shift+Tab` | Previous Field | Move to previous input field |

#### Form Navigation

| Key | Action | Description |
|-----|--------|-------------|
| `Tab` | Next Field | Move to next form field |
| `Shift+Tab` | Previous Field | Move to previous form field |
| `Space` | Toggle | Toggle checkbox/radio options |
| `Enter` | Select/Submit | Select dropdown option or submit form |
| `Esc` | Cancel | Cancel form and return |
| `Ctrl+S` | Submit | Submit form (alternative to Enter) |

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/slayer/gcon.git
cd gcon

# Build the binary
make build

# Run directly
make run

# Run with race detector (for development)
make dev

# Build for all platforms (linux, macOS, windows)
make build-all
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...
```

### Code Quality

```bash
# Install golangci-lint
make install-lint

# Run linter
make lint

# Run linter with auto-fix
make lint-fix
```

### Project Structure

```
gcon/
├── cmd/gcon/              # Application entry point
├── internal/
│   ├── gcp/               # GCP API clients and abstractions
│   │   ├── client.go      # Base client with authentication
│   │   ├── compute.go     # Compute Engine operations
│   │   ├── storage.go     # Cloud Storage operations
│   │   ├── monitoring.go  # Cloud Monitoring (metrics)
│   │   ├── logging.go     # Cloud Logging
│   │   └── metadata.go    # Metadata and labels
│   └── ui/
│       ├── app.go         # Main application controller
│       ├── keys.go        # Global key bindings
│       ├── styles.go      # UI styling (Lip Gloss)
│       ├── components/    # Reusable UI components
│       │   ├── spinner.go
│       │   ├── statusbar.go
│       │   ├── sidebar.go
│       │   └── table.go
│       └── views/         # Feature-specific views
│           ├── projects.go
│           ├── instances.go
│           ├── disks.go
│           ├── snapshots.go
│           ├── images.go
│           ├── buckets.go
│           └── objects.go
├── doc/                   # Feature documentation
├── Makefile
└── README.md
```

## Troubleshooting

### Authentication Issues

**Problem**: `Error: unable to find default credentials`

**Solution**: Run authentication command:
```bash
gcloud auth application-default login
```

Or set up a service account:
```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

### Permission Issues

**Problem**: `Error: Permission denied when listing resources`

**Solution**: Ensure your account has the required IAM roles:
- **Viewer** role for read-only access
- **Compute Admin** for managing Compute Engine resources
- **Storage Admin** for managing Cloud Storage

Grant roles via the GCP Console or using `gcloud`:
```bash
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="user:EMAIL" \
  --role="roles/compute.admin"
```

### Network/Proxy Issues

**Problem**: `Error: connection timeout when calling GCP APIs`

**Solution**: Configure proxy settings:
```bash
export HTTP_PROXY=http://proxy.example.com:8080
export HTTPS_PROXY=http://proxy.example.com:8080
```

Or bypass proxy for GCP APIs:
```bash
export NO_PROXY=googleapis.com,google.com
```

### Terminal Display Issues

**Problem**: Colors or formatting look broken

**Solution**:
1. Ensure your terminal supports 256 colors or truecolor
2. Try disabling colors:
   ```bash
   NO_COLOR=1 gcon
   ```
3. Update your terminal emulator to a modern version

### Performance Issues

**Problem**: Slow loading or high memory usage

**Solution**:
- Use project filtering to limit scope
- Reduce the number of resources in lists
- Check your internet connection to GCP APIs
- Consider using a service account with minimal permissions

## Contributing

We welcome contributions! Here's how you can help:

1. **Fork the repository** on GitHub
2. **Create a feature branch**: `git checkout -b feature/amazing-feature`
3. **Make your changes** following the code style guidelines
4. **Run tests**: `make test`
5. **Run linter**: `make lint`
6. **Commit your changes**: `git commit -m 'Add amazing feature'`
7. **Push to your fork**: `git push origin feature/amazing-feature`
8. **Open a Pull Request**

### Code Style

- Follow Go best practices and idioms
- Use `gofmt` for formatting (included in `make lint`)
- Write meaningful commit messages
- Add tests for new features
- Update documentation when needed
- Keep functions small and focused (<50 lines)
- Use descriptive variable names

### Development Guidelines

- See [CLAUDE.md](CLAUDE.md) for detailed architecture documentation
- Use the Bubble Tea patterns described in the project
- Follow existing UI/UX patterns for consistency
- Add keyboard shortcuts for all new actions
- Show loading spinners for async operations
- Handle errors gracefully with retry options

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) - A powerful TUI framework
- Styled with [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions for terminal output
- Uses [Bubbles](https://github.com/charmbracelet/bubbles) - Common TUI components
- Inspired by [k9s](https://k9scli.io/) and [lazygit](https://github.com/jesseduffield/lazygit)

## License

MIT License - see [LICENSE](LICENSE) file for details

## Support

- **Issues**: Report bugs or request features on [GitHub Issues](https://github.com/slayer/gcon/issues)
- **Discussions**: Join conversations on [GitHub Discussions](https://github.com/slayer/gcon/discussions)
- **Documentation**: See [CLAUDE.md](CLAUDE.md) for technical documentation

---

<p align="center">
  <strong>Made with ❤️ for the Google Cloud community</strong>
</p>
