<p align="center">
  <img src="https://github.com/slayer/gcon/releases/download/v0.0.0-demos/logo.png" alt="GCon logo" width="290">
</p>

<p align="center">
  <strong>A modern terminal UI for managing Google Cloud Platform resources</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/go-1.26+-blue" alt="Go Version">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="License"></a>
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20windows-lightgrey" alt="Platform">
</p>

---

<p align="center">
  <a href="#installation">Installation</a> ·
  <a href="#features">Features</a> ·
  <a href="#authentication">Authentication</a> ·
  <a href="#quick-start">Quick Start</a> ·
  <a href="#usage">Usage</a> ·
  <a href="#development">Development</a> ·
  <a href="#contributing">Contributing</a>
</p>

<p align="center">
  <img src="https://github.com/slayer/gcon/releases/download/v0.0.0-demos/hero.gif" alt="gcon demo" width="800">
</p>

## Overview

**gcon** is a powerful, keyboard-driven terminal user interface (TUI) for managing Google Cloud Platform resources. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and Go, it provides a fast, intuitive alternative to the GCP Console and `gcloud` CLI for common cloud operations.

**Why gcon?**

- 🚀 **Fast & Efficient** - Navigate your GCP resources with keyboard shortcuts, no mouse needed
- 💻 **Terminal Native** - Works seamlessly in any terminal, perfect for SSH sessions and remote work
- 🎨 **Beautiful UI** - Modern terminal interface with GCP color scheme, spinners, and status indicators
- ⚡ **Async Operations** - Non-blocking API calls with real-time loading indicators
- 🔍 **Fuzzy Search** - Quickly find projects, instances, and resources with built-in filtering
- 📊 **Resource Monitoring** - Real-time metrics, logs, and observability for your instances
- 🗂️ **File-manager-grade GCS browser** - `Space` to multi-select, `←`/`→` to navigate folders, bulk delete / download / storage-class change with progress overlays — works the way `mc` / `ranger` users expect

## Demo

### Navigation & Resource Browsing

Browse projects, instances, and resources with keyboard-driven navigation and sidebar.

<p align="center">
  <img src="https://github.com/slayer/gcon/releases/download/v0.0.0-demos/hero.gif" alt="Navigation demo" width="800">
</p>

### Instance Observability

Real-time CPU, memory, and network metrics with braille charts and configurable time ranges.

<p align="center">
  <img src="https://github.com/slayer/gcon/releases/download/v0.0.0-demos/observability.gif" alt="Observability demo" width="800">
</p>

### Logs Explorer

Query, filter, and explore Cloud Logging entries with severity filtering and syntax colorization.

<p align="center">
  <img src="https://github.com/slayer/gcon/releases/download/v0.0.0-demos/logs.gif" alt="Logs Explorer demo" width="800">
</p>

### Command Palette

Fuzzy search to quickly navigate between any resource view.

<p align="center">
  <img src="https://github.com/slayer/gcon/releases/download/v0.0.0-demos/command-palette.gif" alt="Command Palette demo" width="800">
</p>

## Quick Start

1. **Install gcon** (see [Installation](#installation) below)
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

## Installation

### Homebrew (macOS & Linux)

```bash
brew install slayer/gcon/gcon
```

### Install Script (macOS & Linux)

```bash
curl -sSL https://raw.githubusercontent.com/slayer/gcon/master/install.sh | sh
```

Or a specific version:

```bash
curl -sSL https://raw.githubusercontent.com/slayer/gcon/master/install.sh | sh -s -- v0.7.0
```

### Deb / RPM (Linux)

Download `.deb` or `.rpm` packages from the [latest release](https://github.com/slayer/gcon/releases/latest):

```bash
# Debian/Ubuntu
sudo dpkg -i gcon_*.deb

# RHEL/Fedora
sudo rpm -i gcon_*.rpm
```

### From Source

```bash
go install github.com/slayer/gcon/cmd/gcon@latest
```

### From Releases

Download the latest binary from [Releases](https://github.com/slayer/gcon/releases).

### Development setup

The repository ships a [`mise`](https://mise.jdx.dev/) config (`mise.toml`)
pinning the Go and `golangci-lint` versions used by CI. With mise installed:

```bash
mise install        # installs the pinned toolchain
make build          # build the binary
make test           # run all tests
make lint           # run golangci-lint
```

Without mise, install Go 1.26+ and `golangci-lint` 2.12+ manually.

## Features

### ✅ Currently Implemented

<details>
<summary><strong>Project Management</strong></summary>

- 🔄 **Project Selector** - Browse all accessible GCP projects with fuzzy search and filtering
- 🔀 **Quick Project Switching** - Switch between projects via command palette without losing context
- 📋 **Project Metadata** - View and edit project metadata and labels
- 🎯 **Default Project** - Automatic detection from gcloud config or environment variables
</details>

<details>
<summary><strong>Compute Engine</strong></summary>

- 📦 **VM Instance Management**
  - List all instances with status indicators (running, stopped, transitioning)
  - View detailed instance information (machine type, zones, IPs, tags, labels)
  - Start, stop, reset, suspend, and resume instances
  - Delete instances with confirmation dialogs
  - Create new instances with guided forms
  - Real-time status updates with visual indicators
  - **SSH** to running instances via `t` key — options dialog for `gcloud compute ssh` (IAP tunnel, internal IP, user, port forward); falls back to plain `ssh` when gcloud is absent. The session takes over the terminal and returns on exit.

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
</details>

<details open>
<summary><strong>Cloud Storage</strong> — full file-manager UX in your terminal</summary>

> The Cloud Storage views are the most polished part of gcon. The
> Objects browser feels like a real file manager (Far/Midnight Commander
> style) — keyboard-driven, multi-select, parent navigation, bulk
> operations — backed by GCS instead of a local filesystem.

- 🪣 **Bucket Management**
  - List all buckets with inline usage column (total size + object count) sourced from Cloud Monitoring — no extra clicks
  - Bucket details view (`i` key) with **Details** and **Usage** tabs
  - **Deep scan** on demand (`C` key): footer spinner reports
    progress while you keep working; results break down by storage
    class, top-level prefix, and file extension. `Ctrl+X` cancels.
  - Create buckets with the full GCS option set: region / dual /
    multi-region, storage class, uniform vs fine-grained access,
    versioning, retention, soft delete, labels, and CMEK encryption

- 📁 **Object Browser**
  - **Navigate like a shell:** `..` row at the top of every subfolder,
    `←` to go up, `→` to drill in, `Enter` for "open or up", and `Esc`
    pops a back-stack so the previous folder is one keystroke away
  - **Multi-select & bulk actions** — `Space` toggles, `*` selects-all
    on visible (filtered) rows, status bar shows `[N selected]`. Then:
    - `D` — bulk delete with single confirmation (folders auto-expand)
    - `d` — bulk download with per-file progress overlay
    - `.` → *Change storage class* — pick STANDARD / NEARLINE /
      COLDLINE / ARCHIVE; server-side rewrite per object
  - **Rich object details** with KMS key, component count, MD5/CRC32C,
    Public/Authenticated/`gs://` URLs side-by-side, and a dedicated
    **Lifecycle & Retention** section that surfaces:
    - The estimated next lifecycle action with effective date and the
      matching rule ("Delete in 12 days — age ≥ 60d")
    - Per-object retention (Locked/Unlocked + retain-until)
    - Event-based and Temporary holds — warn-styled
    - Custom time used by lifecycle rules
  - **Inline preview** for text files (`v` key) with line numbers and
    truncation hint; `o` opens binary files with the OS default app
  - **Upload** any local file or folder, **download** to the working
    directory, **infinite scroll** through large buckets, and
    **folder-scoped deep scan** (`C`) for one-folder stats with a `✓`
    marker on scanned cells

</details>

<details>
<summary><strong>Cloud SQL</strong></summary>

- 🗄️ **SQL Instance Management**
  - List all Cloud SQL instances with version, state, region, tier, and IP
  - View instance details with 3 tabs (Details/Databases/Backups)
  - Lifecycle actions: start, stop, restart, delete (with type-to-confirm)
  - State display reconciling activationPolicy with instance state
  - List databases per instance
  - List backup runs and create on-demand backups
</details>

<details>
<summary><strong>VPC Networking</strong></summary>

- 🌐 **VPC Network Management**
  - List all VPC networks with subnet counts and routing mode
  - View network details (MTU, routing mode, subnet mode, peerings)
  - IPv6 configuration and ULA range information
  - Browse subnets within each network with IP ranges and regions
  - Navigate between related network resources

- ⚖️ **Load Balancers** (Phase 1 + 2)
  - List forwarding rules across global + all regions, with derived type
    (HTTPS external / internal, HTTP, TCP/SSL proxy, Network LB)
  - Details view with four tabs: Overview, Routing (URL map host/path → backend),
    Backends (instance groups / NEGs, health checks, balancing mode), Observability
  - Live backend health: `● N/M healthy` badges on the Backends tab; expand with
    Tab/Enter for per-instance HEALTHY / UNHEALTHY / DRAINING state and IP:port
  - Serverless NEGs (Cloud Run / Cloud Functions / App Engine) auto-detected and
    shown with a labeled placeholder instead of health polling
  - Observability tab (HTTP/HTTPS LBs): request count, latency (p50/p95/p99),
    error rate (4xx/5xx %), backend latency, throughput (bytes in/out);
    time-range selector (1h–30d), auto-refresh (30 s), manual `r` refresh
  - Delete with dependency cascade (proxy → URL map → backend services → health
    checks), preserving shared resources; type-to-confirm dialog shows exactly
    what will be deleted and what will be kept

- 🔥 **Firewall Rules**
  - List all firewall rules with direction, priority, action, protocols, and status
  - View detailed rule information (allowed/denied entries, source/destination ranges, tags)
  - Enable and disable firewall rules
  - Delete rules with type-to-confirm dialogs
  - Navigate to associated VPC networks from rule details
</details>

<details>
<summary><strong>IAM & Admin</strong></summary>

- 🔐 **Service Account Management**
  - List service accounts with status indicators (active/disabled)
  - View details with keys tab
  - Create, delete, enable/disable service accounts
  - Key management (create/delete) with JSON download on creation

- 📜 **IAM Policy Bindings**
  - View bindings by member or by role with tab navigation
  - Add/remove members to/from role bindings
  - Detail overlay for viewing member's roles or role's members
  - Safe etag-based read-modify-write with conflict retry

- 🎭 **Custom Roles** (read-only)
  - List custom roles with details and permissions tabs
</details>

<details>
<summary><strong>Cloud Run</strong></summary>

- 🚀 **Service Management**
  - List services across all regions with status, URL, latest revision
  - Service details with 4 tabs (Details/Revisions/YAML/Observability)
  - Traffic split editing (validates percentages sum to 100)
  - Edit existing service configuration with diff preview before deploy
  - Create new services with full form (container, scaling, networking, security)
  - Delete services with type-to-confirm

- 📈 **Cloud Run Observability**
  - Request count, latency (p50/p95/p99), error rate (4xx/5xx) charts
  - CPU utilization, billable instance time, instance count charts
  - Filterable log viewer (INFO/WARNING/ERROR severity toggles)
  - Time range selection and auto-refresh
</details>

<details>
<summary><strong>Kubernetes Engine</strong></summary>

- [x] Kubernetes Engine (Phase 1): clusters list + details (Overview, Node Pools)
       + delete with type-to-confirm
</details>

<details>
<summary><strong>Cloud Logging</strong></summary>

- 📋 **Logs Explorer**
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
  - Open in `$PAGER` with `p` key (respects color toggle)
  - Export to TXT/CSV/JSONL via action menu
  - ANSI-aware truncation and wrapping (preserves existing log colors)
</details>

<details>
<summary><strong>User Interface & Navigation</strong></summary>

- 🎯 **Command Palette** - Quick access to all actions with fuzzy search (`:` or `Ctrl+K`)
- 📂 **Sidebar Navigation** - Collapsible sidebar with auto-hide mode and resource categories
- 🍞 **Breadcrumb Navigation** - Always know where you are in the resource hierarchy
- 📜 **Recent Items** - Quick access to recently viewed resources
- ⌨️ **Keyboard Shortcuts** - Vim-style navigation throughout the interface
- 🔄 **Context Menus** - Action menus for resource-specific operations
- ⚡ **Loading States** - Spinners and progress indicators for all async operations
- ❌ **Error Handling** - Inline error display with retry options
- 📊 **Status Bar** - Real-time status updates and operation feedback
</details>

### 🚧 Planned Features

The following features are planned for future releases:

- [ ] **SSH Integration** - Direct SSH access to instances via gcloud
- [ ] **Resource Caching** - Local caching layer for faster repeated queries
- [ ] **Google Kubernetes Engine (GKE) — Phase 2 and beyond**
  - Cluster observability tab, node pool create/delete, resize, upgrade
  - Cluster create wizard (Phase 3)
- [ ] **Cloud Functions** - Function deployment and monitoring
- [ ] **Subnets** - Standalone subnet list and management
- [ ] **Cost Explorer** - Resource cost analysis and budgets

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
| `[` | Focus Sidebar | Move focus to sidebar (expands if auto-hidden) |
| `]` | Focus Content | Move focus to main content (collapses if auto-hide) |
| `{` | Pin Sidebar | Toggle auto-hide / always-open mode |

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
| `C` | Calculate Usage | Deep scan for size/object count (footer spinner, `Ctrl+X` to cancel) |
| `D` | Delete | Delete bucket (must be empty) |
| `/` | Filter | Filter buckets by name |
| `r` | Refresh | Reload bucket list |

#### Cloud Storage - Objects

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | Open/View | Open folder, view file details, or go up (on `..` row) |
| `→` | Enter Folder | Drill into the selected folder (no-op on files and on `..`) |
| `←` | Go Up | Navigate to parent folder (no-op at bucket root) |
| `Space` | Toggle Select | Add/remove cursor row from the bulk selection |
| `*` | Select All | Toggle select-all on visible (filtered) rows |
| `u` | Upload | Upload files to current folder |
| `d` | Download | Download cursor row, or **all selected** when a selection is active |
| `D` | Delete | Delete cursor row, or **all selected** when a selection is active |
| `.` | Action Menu | Per-row actions, or **bulk menu** (incl. *Change storage class*) when a selection is active |
| `Esc` | Clear / Back | Clear bulk selection if any; otherwise pop navigation history |

#### VPC Networks

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | View Details | Show detailed network information |
| `/` | Filter | Filter networks by name |
| `r` | Refresh | Reload network list |
| `Esc` | Go Back | Return to previous view |

#### VPC Network Details

| Key | Action | Description |
|-----|--------|-------------|
| `.` | Action Menu | Open context-sensitive action menu |
| `r` | Refresh | Refresh network details and subnets |
| `Tab` | Switch Focus | Switch focus between tabs, links, and content |
| `h`/`l` or `1`/`2` | Switch Tabs | Switch between Details and Subnets tabs |
| `j`/`k` or `↓`/`↑` | Navigate | Navigate subnet links or scroll content |
| `Esc` | Go Back | Return to networks list |

#### Firewall Rules

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | View Details | Show detailed firewall rule information |
| `.` | Action Menu | Open context-sensitive action menu |
| `t` | Toggle | Enable or disable firewall rule |
| `D` | Delete | Delete firewall rule (with confirmation) |
| `/` | Filter | Filter rules by name, direction, network |
| `r` | Refresh | Reload firewall rules list |
| `Esc` | Go Back | Return to previous view |

#### Firewall Rule Details

| Key | Action | Description |
|-----|--------|-------------|
| `.` | Action Menu | Open context-sensitive action menu |
| `t` | Toggle | Enable or disable firewall rule |
| `D` | Delete | Delete firewall rule (with confirmation) |
| `r` | Refresh | Refresh rule details |
| `Tab` | Switch Focus | Switch focus between tabs, links, and content |
| `h`/`l` or `1`/`2` | Switch Tabs | Switch between Details and Rules tabs |
| `Enter` | Navigate | Navigate to associated VPC network |
| `Esc` | Go Back | Return to firewall rules list |

#### Cloud SQL - Instances

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | View Details | Show instance details with tabs |
| `.` | Action Menu | Open context-sensitive action menu |
| `S` | Sort Menu | Open column sort menu |
| `s` | Start | Start a stopped instance |
| `x` | Stop | Stop a running instance |
| `R` | Restart | Restart an instance |
| `D` | Delete | Delete instance (with type-to-confirm) |
| `/` | Filter | Filter instances |
| `r` | Refresh | Reload instance list |
| `Esc` | Go Back | Return to previous view |

#### Cloud SQL - Instance Details

| Key | Action | Description |
|-----|--------|-------------|
| `.` | Action Menu | Open context-sensitive action menu |
| `s` | Start | Start instance (if stopped) |
| `x` | Stop | Stop instance (if running) |
| `R` | Restart | Restart instance |
| `D` | Delete | Delete instance (with type-to-confirm) |
| `b` | Create Backup | Create on-demand backup (Backups tab) |
| `r` | Refresh | Refresh all tabs |
| `Tab` | Switch Focus | Switch focus between tabs and content |
| `h`/`l` or `1`/`2`/`3` | Switch Tabs | Switch between Details, Databases, and Backups tabs |
| `j`/`k` or `↓`/`↑` | Scroll | Scroll content |
| `Esc` | Go Back | Return to instances list |

#### IAM Policy

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | Details | Open member/role detail overlay |
| `a` | Add | Add member to role or role to member |
| `.` | Action Menu | Open context-sensitive action menu |
| `S` | Sort Menu | Open column sort menu |
| `/` | Filter | Filter by role, member, or condition |
| `r` | Refresh | Reload IAM policy |
| `Tab` | Switch Focus | Switch focus between tabs and table |
| `h`/`l` or `1`/`2` | Switch Tabs | Switch between By Member and By Role tabs |
| `Esc` | Close/Back | Close overlay, clear filter, or go back |

##### IAM Policy Overlay

| Key | Action | Description |
|-----|--------|-------------|
| `j`/`k` or `↓`/`↑` | Navigate | Navigate items in the overlay |
| `a` | Add | Add member or role |
| `d` | Remove | Remove selected item (with confirmation) |
| `Esc` | Close | Close overlay |

#### Cloud Run Services

| Key | Action | Description |
|-----|--------|-------------|
| `Enter` | View Details | Show service details |
| `.` | Action Menu | Open context-sensitive action menu |
| `c` | Create | Create a new service |
| `D` | Delete | Delete service (with type-to-confirm) |
| `e` | Edit | Edit service configuration |
| `/` | Filter | Filter services |
| `r` | Refresh | Reload service list |

#### Logs Explorer

| Key | Action | Description |
|-----|--------|-------------|
| `/` | Focus Query | Focus the LQL query input |
| `Enter` | Run/Expand | Run query (input) / Expand entry / Filter by field |
| `Esc` | Blur/Collapse | Blur input / Collapse entry / Close filter / Go back |
| `Tab` | Cycle Focus | Cycle focus (entries / filters / query / time range) |
| `Shift+Tab` | Cycle Back | Cycle focus backwards |
| `j/k` or `↓/↑` | Navigate | Navigate log entries |
| `→` or `Enter` | Expand | Expand entry / Enter field navigation |
| `←` | Collapse | Collapse entry / Exit field navigation |
| `PgUp/PgDn` | Page | Page up/down through entries |
| `E` | Expand All | Expand all visible entries |
| `C` | Collapse All | Collapse all entries |
| `w` | Toggle Wrap | Toggle line wrapping |
| `c` | Toggle Colors | Toggle logfmt/protobuf colorization |
| `1-5` | Time Range | Set time range (1h/6h/24h/7d/30d) |
| `f` | Tail Mode | Toggle tail mode (15s polling) |
| `r` | Refresh | Re-run query |
| `R` | Resources | Open resource type filter |
| `L` | Log Names | Open log name filter |
| `V` | Severities | Open severity filter |
| `p` | Pager | Open entries in `$PAGER` |
| `.` | Action Menu | Export to TXT/CSV/JSONL |

#### Logs Explorer - Filter Dropdown

| Key | Action | Description |
|-----|--------|-------------|
| `j/k` or `↓/↑` | Navigate | Navigate filter options |
| `PgUp/PgDn` | Page | Page up/down through options |
| `Space/Tab/Enter` | Toggle | Toggle option selection |
| `/` | Search | Search within filter options |
| `Esc` | Apply/Close | Apply selection and close |

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
│   │   ├── logging.go     # Cloud Logging (entries, filters, histogram)
│   │   ├── metadata.go    # Metadata and labels
│   │   ├── networks.go    # VPC Networks and subnets
│   │   ├── firewalls.go   # Firewall rules
│   │   ├── sql.go         # Cloud SQL instances
│   │   ├── iam.go         # IAM policies, service accounts
│   │   └── cloudrun.go    # Cloud Run services
│   └── ui/
│       ├── app.go         # Main application controller
│       ├── keys.go        # Global key bindings
│       ├── styles.go      # UI styling (Lip Gloss)
│       ├── components/    # Reusable UI components
│       │   ├── spinner.go
│       │   ├── statusbar.go
│       │   ├── sidebar.go
│       │   ├── table.go
│       │   └── logviewer/ # Log entry list with expand/collapse
│       └── views/         # Feature-specific views
│           ├── projects.go
│           ├── instances.go
│           ├── disks.go
│           ├── snapshots.go
│           ├── images.go
│           ├── buckets.go
│           ├── objects.go
│           ├── networks.go
│           ├── network_details.go
│           ├── firewalls.go
│           ├── firewall_details.go
│           ├── sql_instances.go
│           ├── sql_instance_details.go
│           ├── cloudrun_services.go
│           └── logs.go    # Cloud Logging Explorer
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
