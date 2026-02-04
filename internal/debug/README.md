# Debug Package

The `internal/debug` package provides debug logging functionality for gcon.

## Usage

Enable debug logging by passing the `--debug` flag when running gcon:

```bash
./gcon --debug
```

This creates a `gcon-debug.log` file in the current working directory. The log file is synchronized after every write, which impacts performance but ensures logs are captured even if the application crashes.

## API

### EnableDebug() error

Enables debug logging and creates the log file. Returns an error if the file cannot be created.

```go
if err := debug.EnableDebug(); err != nil {
    fmt.Fprintf(os.Stderr, "Failed to enable debug: %v\n", err)
}
```

### Close() error

Closes the debug log file and disables logging. Safe to call multiple times.

```go
defer debug.Close()
```

### IsEnabled() bool

Returns true if debug logging is currently enabled.

```go
if debug.IsEnabled() {
    // Do expensive debug work
}
```

### Log(format string, args ...interface{})

Writes a formatted message to the debug log. No-op if debug is disabled.

```go
debug.Log("Processing item: %s (count=%d)", item.Name, item.Count)
```

### LogView(name string, content string)

Logs information about a rendered view, useful for debugging layout issues in Bubble Tea components.

```go
content := myView.View()
debug.LogView("MyView", content)
```

## Performance Warning

Debug logging is **slow** because:
1. Every log message is written to disk immediately (no buffering)
2. Each write is followed by an explicit `Sync()` call to flush OS buffers
3. This is intentional to ensure logs are captured during crashes

Only use `--debug` when actively troubleshooting issues. Do not enable it in production or for normal usage.

## Log Location

The log file is created in the **current working directory** as `gcon-debug.log`, not in `/tmp`. This makes it easy to find after the application exits.

## Testing

Run tests with:

```bash
go test ./internal/debug/... -v
```

The tests verify:
- Log file creation in current directory
- Logging when enabled/disabled
- Multiple log writes
- View logging functionality
- Proper cleanup with Close()
