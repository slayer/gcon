# Golden File Visual Regression Tests — Design

## Overview

Golden file snapshot testing for gcon's TUI views. Compares `View()` output (with full ANSI
escape codes) against committed `.golden` files. Catches layout breaks, missing content,
style regressions, and overlay z-order bugs.

## Architecture

```
View under test
    ↓
Inject fixture data via messages
    ↓
Call View() → raw string with ANSI codes
    ↓
golden.RequireEqual(t, []byte(output))
    ↓
Compare against testdata/TestName.golden
    ↓
Pass (match) or Fail (unified diff)
```

### Why `golden` and not `teatest`

`teatest` wraps a full `tea.Program` lifecycle — it spins up the program, captures live output,
and waits for quit. gcon's views are sub-models managed by `App`, not standalone `tea.Model`s.
Using `teatest` would require either:
- Testing through `App` for everything (slow, brittle, tests too much at once)
- Making each view a standalone `tea.Model` (architectural change)

The `golden` package gives us just the comparison engine. We call `View()` directly and compare.

### Why full ANSI colors

`termenv.TrueColor` mode captures lipgloss styling in golden files. This catches:
- Color changes (wrong status indicator color)
- Style changes (bold/faint/underline regressions)
- Background color bleeding (the transparent background bug documented in rendering rules)

Trade-off: golden file diffs in PRs contain escape sequences. Mitigated by `go test -update`
workflow — reviewers check test output, not raw golden file content.

## Test Helper Infrastructure

```go
// internal/ui/views/golden_test_helpers.go

func init() {
    lipgloss.SetColorProfile(termenv.TrueColor)
}

const (
    goldenWidth  = 120
    goldenHeight = 40
)

func sendKeys(view ViewModel, keys ...tea.KeyMsg) tea.Cmd {
    var lastCmd tea.Cmd
    for _, k := range keys {
        lastCmd = view.Update(k)
    }
    return lastCmd
}

func key(r rune) tea.KeyMsg {
    return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func specialKey(t tea.KeyType) tea.KeyMsg {
    return tea.KeyMsg{Type: t}
}
```

## Test Patterns

### View Snapshot (basic)

```go
func TestInstancesView_Loaded(t *testing.T) {
    view := NewInstancesView(nil, "my-project")
    view.SetSize(goldenWidth, goldenHeight)
    view.Update(instancesLoadedMsg{instances: testInstances()})

    golden.RequireEqual(t, []byte(view.View()))
}
```

### Loading State (spinner determinism)

```go
func TestInstancesView_Loading(t *testing.T) {
    view := NewInstancesView(nil, "my-project")
    view.SetSize(goldenWidth, goldenHeight)
    view.Update(view.spinner.Tick())  // freeze at known frame

    golden.RequireEqual(t, []byte(view.View()))
}
```

### Dialog Overlay

```go
func TestInstancesView_ActionMenu(t *testing.T) {
    view := NewInstancesView(nil, "my-project")
    view.SetSize(goldenWidth, goldenHeight)
    view.Update(instancesLoadedMsg{instances: testInstances()})
    sendKeys(view, key('.'))  // open action menu

    golden.RequireEqual(t, []byte(view.View()))
}
```

### Confirm Dialog with Text Input

```go
func TestInstancesView_DeleteConfirm(t *testing.T) {
    view := NewInstancesView(nil, "my-project")
    view.SetSize(goldenWidth, goldenHeight)
    view.Update(instancesLoadedMsg{instances: testInstances()})
    sendKeys(view, key('D'))
    view.Update(textinput.Blink())  // cursor visible

    golden.RequireEqual(t, []byte(view.View()))
}
```

### App-Level (full layout)

```go
func TestApp_InstancesWithSidebar(t *testing.T) {
    app := createTestApp()
    app.SetSize(goldenWidth, goldenHeight)
    app.Update(projectSelectedMsg{project: testProject()})
    app.Update(instancesLoadedMsg{instances: testInstances()})

    golden.RequireEqual(t, []byte(app.View()))
}
```

## Determinism Strategies

| Source of non-determinism | Strategy |
|--------------------------|----------|
| Spinner frame | Call `spinner.Tick()` fixed number of times |
| Cursor blink | Send `textinput.Blink()` to ensure visible |
| Terminal color profile | Force `termenv.TrueColor` in `init()` |
| Terminal size | Fixed `120x40` constant |
| Map iteration order | Already sorted in production code (per component-patterns rule) |
| Timestamps in data | Use fixed timestamps in fixture structs |

## File Organization

```
internal/ui/
├── views/
│   ├── golden_test_helpers.go
│   ├── instances_golden_test.go
│   ├── instance_details_golden_test.go
│   ├── buckets_golden_test.go
│   ├── ...
│   └── testdata/
│       ├── TestInstancesView_Loaded.golden
│       ├── TestInstancesView_ActionMenu.golden
│       └── ...
├── app_golden_test.go
├── testdata/
│   ├── TestApp_InstancesWithSidebar.golden
│   └── ...
```

## Git Configuration

`.gitattributes`:
```
*.golden -text
```

Prevents Git from modifying line endings in golden files.

## Makefile Targets

```makefile
test-golden:
	go test -v ./internal/ui/... -run Golden

test-golden-update:
	go test ./internal/ui/... -run Golden -update

test-golden-ci:
	go test ./internal/ui/... -run Golden
```

## PR Workflow

1. Make rendering change
2. Run `make test-golden-update`
3. `git diff` the `.golden` files to review visual impact
4. Commit golden files alongside code changes
5. CI runs `make test-golden-ci` — fails if golden files are stale

## Coverage Targets

~4-6 golden files per view:

| Snapshot | Purpose |
|----------|---------|
| Loaded | Normal rendering with data |
| Loading | Spinner state |
| Error | Error display |
| Empty | No data state |
| Dialog 1 | Action menu or confirm |
| Dialog 2 | Secondary dialog if applicable |

## What NOT to Snapshot

- Observability charts with live timestamps
- Every sort/filter permutation
- Every scroll position
- Intermediate animation frames
