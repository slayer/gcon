# Default GCP Project Support

## Task Description

Add support for automatically detecting and using the default GCP project with the following priority:

1. **CLI flag** (`-p` / `--project`) - highest priority
2. **Environment variable** (`CLOUDSDK_CORE_PROJECT`)
3. **gcloud config** - parse active configuration files
4. **Fallback** - show project selection dialog (current behavior)

## Implementation Plan

- [x] Step 1: Create `internal/config/gcloud.go` - gcloud config file parser
- [x] Step 2: Create `internal/config/resolver.go` - project resolution logic
- [x] Step 3: Modify `internal/ui/messages.go` - add InitialProjectLoadedMsg/ErrorMsg
- [x] Step 4: Modify `internal/ui/app.go` - add AppOptions, initial project handling
- [x] Step 5: Modify `cmd/gcon/main.go` - add flag parsing, integrate resolver
- [x] Step 6: Write tests for config package
- [x] Step 7: Run full test suite and fix any issues
- [x] Step 8: Create documentation

## Technical Details

### gcloud Config File Locations

- Config dir: `~/.config/gcloud` (or `CLOUDSDK_CONFIG` env var)
- Active config indicator: `{configDir}/properties` contains `[core]\nconfig = <name>`
- Config files: `{configDir}/configurations/config_{name}`

### INI Format

```ini
[core]
account = user@example.com
project = my-project-id
```

## Edge Cases

- gcloud not installed (no config dir) -> show selector
- Config file exists but no project set -> show selector
- Project ID in config but project deleted/no access -> error + selector fallback
- Multiple gcloud configurations -> use active one
- `CLOUDSDK_CONFIG` env var set -> use custom config dir
