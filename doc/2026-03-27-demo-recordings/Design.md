# Demo GIF Recordings for gcon

## Goal

Create polished demo GIFs for the README showing gcon's main features, recorded with VHS against real GCP resources.

## Tooling

- **VHS** (`github.com/charmbracelet/vhs`) — declarative terminal recorder
- **gcloud CLI** — for setup/teardown of demo resources
- Scripts read environment from `.envrc` (direnv)

## File Structure

```
demos/
├── README.md              # Recording instructions & workflow
├── .envrc                 # Demo-specific env vars (project, SA, etc.)
├── setup.sh               # Create demo GCP resources
├── teardown.sh            # Destroy demo GCP resources
├── hero.tape              # Main showcase GIF
├── observability.tape     # Metrics/charts GIF
├── logs.tape              # Logs Explorer GIF
├── command-palette.tape   # Command palette GIF
└── output/                # Generated GIFs (gitignored)
    ├── hero.gif
    ├── observability.gif
    ├── logs.gif
    └── command-palette.gif
```

## Demo GCP Resources

All resources use `gcon-demo-` prefix for safe teardown.

| Resource | Name | Purpose (GIF) | Cost |
|----------|------|----------------|------|
| e2-micro VM | `gcon-demo-vm` | Instances, details, observability | Free tier / ~$6/mo |
| Cloud Run service | `gcon-demo-svc` | Cloud Run list, observability | Free tier |
| GCS bucket | `gcon-demo-bucket-{project}` | Buckets, objects browser | Negligible |
| Sample objects | `readme.md`, `data/sample.json`, `logs/app.log` | Object navigation, folders | Free |
| Service account | `gcon-demo-sa` | IAM service accounts, keys | Free |
| IAM binding | viewer role on demo SA | IAM policy view | Free |
| Custom role | `gcon.demo.viewer` | Custom roles view | Free |
| Firewall rule | `gcon-demo-allow-http` | Firewall list/details | Free |
| Static route | `gcon-demo-route` | Routes list/details | Free |
| Cloud SQL (micro) | `gcon-demo-sql` | SQL instances, databases, backups | ~$7/mo |

**Total**: Under $1 if resources are up for ~30 min only.

## Environment Variables (.envrc)

```bash
export DEMO_PROJECT="your-gcp-project-id"
export DEMO_REGION="us-central1"
export DEMO_ZONE="us-central1-a"
export DEMO_SA_NAME="gcon-demo-sa"
export DEMO_BUCKET="gcon-demo-bucket-${DEMO_PROJECT}"
```

## GIF Plan

### 1. Hero GIF (~15s)

Flow: Launch → project list → select project → instances list → open details → Esc → command palette → "logs" → Logs Explorer

Shows: project selection, sidebar navigation, details view, command palette.

### 2. Observability GIF (~12s)

Flow: Instance details → Observability tab → CPU/memory charts → change time range → toggle auto-refresh

Shows: real-time metrics, chart rendering, time range selection.

### 3. Logs Explorer GIF (~12s)

Flow: Open Logs Explorer → expand log entry → field navigation → severity filter → toggle colorization

Shows: log querying, entry expansion, filtering, colorization.

### 4. Command Palette GIF (~8s)

Flow: `:` → type "cloud run" → navigate → `:` again → "buckets" → navigate

Shows: fuzzy search, quick navigation between features.

## VHS Settings (shared)

```tape
Set Theme "Catppuccin Mocha"
Set FontSize 14
Set Width 1200
Set Height 700
Set Padding 20
```

Pacing:
- `Sleep 2s` after major UI transitions
- `Sleep 500ms` between keypresses
- `Sleep 1s` as "look at this" pause

## Makefile Targets

```makefile
DEMO_PROJECT ?= $(shell gcloud config get-value project)

demo-setup:
	./demos/setup.sh

demo-teardown:
	./demos/teardown.sh

demos:
	cd demos && for f in *.tape; do vhs $$f; done
```

## Recording Workflow

1. `make demo-setup` — Create resources (~2 min)
2. Wait 10-15 min — Let metrics/logs accumulate
3. `make demos` — Record all GIFs (~2 min)
4. Upload GIFs to GitHub release (`v0.0.0-demos`)
5. Update README.md with GIF URLs
6. `make demo-teardown` — Destroy resources (~3 min)

## GIF Hosting

Upload to a GitHub release as assets, reference by URL in README. Keeps repo lean.
