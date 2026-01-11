# gcon

A terminal UI for managing Google Cloud Platform resources.

![Go Version](https://img.shields.io/badge/go-1.22+-blue)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

## Features

- **Project Selector** - Browse and filter GCP projects with fuzzy search
- **Compute Engine** - List, start, stop, and reset VM instances
- **Keyboard-driven** - Vim-style navigation and shortcuts
- **Fast** - Async API calls with loading indicators

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

## Usage

```bash
gcon
```

### Key Bindings

| Key | Action |
|-----|--------|
| `j/k` or `↓/↑` | Navigate list |
| `Enter` | Select |
| `Esc` | Go back |
| `/` | Search/Filter |
| `r` | Refresh |
| `q` | Quit |

#### Compute Instances

| Key | Action |
|-----|--------|
| `s` | Start instance |
| `x` | Stop instance |
| `R` | Reset instance |

## Building

```bash
# Build
make build

# Run
make run

# Test
make test
```

## License

MIT
