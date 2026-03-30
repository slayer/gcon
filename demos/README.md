# Demo GIF Recordings

Record polished GIFs of gcon's main features using [VHS](https://github.com/charmbracelet/vhs).

## Prerequisites

- [VHS](https://github.com/charmbracelet/vhs) installed (`go install github.com/charmbracelet/vhs@latest`)
- [gcloud CLI](https://cloud.google.com/sdk/docs/install) authenticated
- [direnv](https://direnv.net/) (optional, for automatic `.envrc` loading)
- `gcon` binary built (`make build`)
- `gcon-sa.json` service account key in this directory

## Setup

1. Copy the environment template and fill in your GCP project:

   ```bash
   cp .envrc.example .envrc
   # Edit .envrc with your project ID
   direnv allow  # or: source .envrc
   ```

2. Place your service account key as `gcon-sa.json` in this directory.

3. Create demo resources:

   ```bash
   make demo-setup
   ```

4. Wait 10-15 minutes for metrics and logs to accumulate.

## Recording

Record all GIFs:

```bash
make demos
```

GIFs are written to `output/` (gitignored).

## Teardown

Destroy all demo resources:

```bash
make demo-teardown
```

## Resource Management

Both setup and teardown use `resources.sh`:

```bash
./resources.sh setup     # create all demo resources
./resources.sh teardown  # destroy all demo resources
```

All resources use `gcon-demo-*` prefix. Operations are idempotent.

## Tape Files

| File | Duration | Features shown |
|------|----------|----------------|
| `hero.tape` | ~15s | Project selection, instances, details, command palette |
| `observability.tape` | ~12s | Metrics charts, time range, auto-refresh |
| `logs.tape` | ~12s | Log entries, expansion, severity filter, colorization |
| `command-palette.tape` | ~8s | Fuzzy search, quick navigation between views |

## Customizing

Edit `.tape` files to adjust:
- `Sleep` durations for pacing
- `Set Width`/`Set Height` for dimensions
- `Set Theme` for color scheme (default: Catppuccin Mocha)
- Key sequences for different navigation paths

## GIF Hosting

After recording, upload GIFs to a GitHub release:

```bash
gh release create v0.0.0-demos output/*.gif --title "Demo GIFs" --notes "Demo recordings for README"
```

Then reference them in the README by URL.
