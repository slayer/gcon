# Command Palette Feature Documentation

## Overview

The Command Palette provides a quick, keyboard-driven way to navigate between views and execute actions without using the sidebar. It's inspired by VS Code's command palette and Vim's command mode.

## Usage

### Opening the Palette

- Press `:` (Vim style) - Shows the `:` prefix in the input
- Press `Ctrl+K` (Spotlight style) - Opens with empty input

### Navigation

| Key | Action |
|-----|--------|
| `↑` / `Ctrl+p` | Move selection up |
| `↓` / `Ctrl+n` | Move selection down |
| `Enter` | Execute selected command |
| `Esc` | Close palette |
| Text | Filter commands (fuzzy search) |

### Command Types

1. **Navigation Commands** - Navigate to different views
   - `Compute Engine: VM instances`
   - `Compute Engine: Disks`
   - `Cloud Storage: Buckets`
   - `VPC Network: VPC networks`
   - `VPC Network: Firewall`

2. **Action Commands** - Execute actions
   - `Refresh` - Refresh current view
   - `Toggle sidebar` - Show/hide sidebar
   - `Help` - Show help overlay

3. **Recent Items** - Recently accessed resources
   - Shown at the top of the list
   - Includes projects, buckets, and instances

## Fuzzy Search

The palette supports fuzzy matching:

- **Exact match**: "VM instances" finds "VM instances"
- **Prefix match**: "VM" finds "VM instances"
- **Word boundary match**: "instances" finds "VM instances"
- **Contains match**: "nsta" finds "VM instances"
- **Multi-word**: "ce vm" finds "Compute Engine: VM instances"

Results are ranked by match quality.

## Architecture

### Component Structure

```
internal/ui/components/commandpalette/
├── commandpalette.go      # Main component
├── commandpalette_test.go # Tests
├── commands.go            # Command types and registry
├── fuzzy.go              # Fuzzy matching algorithm
├── fuzzy_test.go         # Fuzzy tests
├── styles.go             # Lip Gloss styling
├── recent.go             # Recent items tracker
└── recent_test.go        # Recent items tests
```

### Key Types

```go
// Command represents an executable palette item
type Command struct {
    ID       string      // Unique identifier
    Label    string      // Display text
    Icon     string      // Unicode icon
    Type     CommandType // Navigation, Action, or Recent
    ViewType ViewType    // Target view for navigation
    Action   func() tea.Cmd // Action to execute
    Enabled  bool        // Whether command can be executed
}
```

### Message Types

- `CommandSelectedMsg` - Emitted when user selects a command
- `CommandCancelMsg` - Emitted when user cancels (Esc)

### Integration Points

1. **app.go**: Manages palette state, handles key bindings, renders overlay
2. **keys.go**: Defines `:` and `Ctrl+K` bindings
3. **Recent tracking**: Hooks into `ProjectSelectedMsg`, `BucketSelectedMsg`, `InstanceSelectedMsg`

## Visual Design

```
┌──────────────────────────────────────────────────┐
│ :vm_                                             │
├──────────────────────────────────────────────────┤
│ ▸ ■ Compute Engine: VM instances                 │  <- selected (blue bg)
│   ◆ VPC Network: VPC networks                    │
└──────────────────────────────────────────────────┘
```

- Centered modal overlay (~60% width)
- Input at top with optional `:` prefix
- Separator line between input and results
- Icon + label for each command
- Selected item: cursor indicator (`▸`) + GCP blue background

## Future Enhancements

- Persist recent items across sessions
- Add more action commands
- Keyboard shortcuts shown next to commands
- Command categories/sections
- Custom user commands
