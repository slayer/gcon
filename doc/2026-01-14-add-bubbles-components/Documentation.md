# Documentation: Bubbles Components

## Summary

Added three new UI components wrapping [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles) with GCP styling:

1. **Viewport** - Scrollable content container
2. **TextArea** - Multi-line text editor
3. **Timer** - Countdown and stopwatch

## Components

### Viewport (`internal/ui/components/viewport/`)

Wraps `bubbles/viewport` for displaying scrollable content like logs or long text.

**Features:**
- Optional title header
- Optional border styling
- Keyboard navigation (j/k, arrows, pgup/pgdn, home/end)
- Mouse wheel scrolling
- Scroll position indicator

**Usage:**
```go
vp := viewport.New(80, 24).
    WithTitle("Log Output").
    WithBorder(true)
vp.SetContent(logContent)
```

**Key bindings:**
| Key | Action |
|-----|--------|
| j/↓ | Scroll down |
| k/↑ | Scroll up |
| ctrl+d/pgdn | Page down |
| ctrl+u/pgup | Page up |
| g/home | Go to top |
| G/end | Go to bottom |

---

### TextArea (`internal/ui/components/textarea/`)

Wraps `bubbles/textarea` for multi-line text editing with GCP styling.

**Features:**
- Line numbers (optional)
- Placeholder text
- Character limit configuration
- Read-only mode for viewing
- Cursor position info

**Usage:**
```go
ta := textarea.New().
    WithTitle("Edit Description").
    WithPlaceholder("Enter text...").
    WithLineNumbers(true).
    WithCharLimit(1000)
ta.SetValue(existingContent)
```

**Read-only mode:**
```go
ta := textarea.New().ReadOnly(true)
ta.SetValue(content)  // Displays content without editing
```

---

### Timer (`internal/ui/components/timer/`)

Wraps `bubbles/timer` for countdown and elapsed time display.

**Features:**
- Countdown mode (counts down to zero)
- Stopwatch mode (counts up from zero)
- Start/stop/toggle/reset operations
- Color changes based on remaining time (warning/error colors)
- Optional label prefix

**Usage - Countdown:**
```go
tm := timer.NewCountdown(5 * time.Minute).
    WithLabel("Time remaining")
tm.Start()  // Returns tea.Cmd for tick updates
```

**Usage - Stopwatch:**
```go
tm := timer.NewStopwatch().
    WithLabel("Elapsed")
tm.Start()
```

**Methods:**
- `Start()` - Begin timer
- `Stop()` - Pause timer
- `Toggle()` - Start or stop
- `Reset()` - Reset to initial state
- `Remaining()` - Get remaining duration (countdown)
- `Elapsed()` - Get elapsed duration
- `TimedOut()` - Check if countdown reached zero

---

## File Structure

```
internal/ui/components/
├── viewport/
│   ├── viewport.go       # Viewport component
│   └── viewport_test.go  # Tests
├── textarea/
│   ├── textarea.go       # TextArea component
│   └── textarea_test.go  # Tests
└── timer/
    ├── timer.go          # Timer component
    └── timer_test.go     # Tests
```

## Integration: Elapsed Time in File Operations

The Timer functionality has been integrated into the existing Progress component to show elapsed time during file transfers.

### Progress Component Updates (`internal/ui/components/progress/progress.go`)

Added elapsed time tracking:
- `Start()` - Begin elapsed time tracking when operation starts
- `Elapsed()` - Get current elapsed duration
- `Reset()` - Reset progress state including timer
- `SetShowElapsed(bool)` - Control elapsed time display

### Usage in Objects View

Elapsed time is automatically displayed during:
- **Downloads** - Shows time spent downloading files
- **Uploads** - Shows time spent uploading files
- **Deletes** - Shows time spent deleting files

The elapsed time appears next to the operation title in MM:SS format (or HH:MM:SS for long operations).

---

## Testing

All components have unit tests covering:
- Initialization and configuration
- State management
- Update handling
- View rendering
- Elapsed time formatting and display

Run tests:
```bash
make test
```

## Current Use Cases

- **Viewport**: Ready for Cloud Logging viewer, instance details with long metadata
- **TextArea**: Ready for editing instance labels, Cloud Function code preview
- **Timer/Elapsed Time**: **Active** - Shows operation duration during file downloads, uploads, and deletes

## Future Use Cases

- **Viewport**: Cloud Logging viewer integration
- **TextArea**: Edit instance labels inline
- **Timer**: API timeout indicators, instance uptime display
