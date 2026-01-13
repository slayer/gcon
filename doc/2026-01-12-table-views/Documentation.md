# Table Views Implementation

## Summary

Converted all list-based views to table-based views using `charmbracelet/bubbles/table` for better columnar data display. Created a reusable table component wrapper with GCP-inspired styling and built-in filtering support.

## Changes Made

### New Components

#### `internal/ui/components/table/table.go`
- Reusable table wrapper around `bubbles/table`
- Built-in filtering via `/` key (textinput component)
- Row selection tracking with ID-based lookup
- Custom `Row` type with `Data`, `FilterValue`, and `ID` fields

#### `internal/ui/components/table/styles.go`
- GCP-inspired color palette
- Table header, cell, and selection styles
- Status-specific styles (running/stopped/pending)

### Modified Views

#### `internal/ui/views/instances.go`
**Columns:** Name | Status | Zone | Internal IP | External IP | Machine Type

- Replaced `bubbles/list` with table component
- Added `instanceToRow()` for converting instances to table rows
- Preserved all keybindings (s=start, x=stop, R=reset, r=refresh, Enter=details)

#### `internal/ui/views/projects.go`
**Columns:** Name | Project ID | State

- Replaced `bubbles/list` with table component
- Added `projectToRow()` and `stateIcon()` functions
- State displayed with emoji indicators

#### `internal/ui/views/buckets.go`
**Columns:** Name | Location | Storage Class | Created

- Replaced `bubbles/list` with table component
- Added `bucketToRow()` function

#### `internal/ui/views/objects.go`
**Columns:** Name | Size | Content Type | Modified

- Replaced `bubbles/list` with table component
- Added `objectToRow()` function
- Preserved folder navigation and pagination

### Test Updates

Updated test files to match new API:
- `buckets_test.go`: Tests `bucketToRow()` function
- `objects_test.go`: Tests `objectToRow()` function

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     View (e.g., InstancesView)              │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────┐    │
│  │              table.Model (wrapper)                  │    │
│  │  ┌─────────────────────────────────────────────┐    │    │
│  │  │          bubbles/table.Model                │    │    │
│  │  └─────────────────────────────────────────────┘    │    │
│  │  ┌─────────────────────────────────────────────┐    │    │
│  │  │          bubbles/textinput.Model            │    │    │
│  │  │          (filtering)                        │    │    │
│  │  └─────────────────────────────────────────────┘    │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## Key Bindings

| Key | Action |
|-----|--------|
| `/` | Enter filter mode |
| `esc` | Exit filter mode / Go back |
| `enter` | Select item / Confirm filter |
| `j/k` or `↓/↑` | Navigate rows |
| `r` | Refresh |

## Testing

```bash
make test   # All tests pass
make lint   # No issues
make build  # Builds successfully
```
