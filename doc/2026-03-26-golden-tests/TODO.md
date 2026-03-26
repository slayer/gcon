# Golden File Visual Regression Tests

## Task Description

Add golden file snapshot testing for TUI views using `github.com/charmbracelet/x/exp/golden`.
Catches visual regressions (broken layouts, missing content, style changes) by comparing
`View()` output against committed `.golden` files with full ANSI color codes.

## Design Decisions

- **`golden` package directly** (not full `teatest`) — views are sub-models of App, not standalone
  `tea.Model`s, so `teatest`'s program lifecycle doesn't fit
- **Full ANSI colors** (`termenv.TrueColor`) — catches style regressions, not just content
- **Spinner determinism** — freeze at known frame via `spinner.Tick()` calls
- **Cursor determinism** — send `textinput.Blink()` to ensure visible state
- **Fixture data as Go structs** — no `--test-data` CLI flag, no mock HTTP servers

## Implementation Plan

### Phase 1: Infrastructure
- [ ] Add `github.com/charmbracelet/x/exp/golden` dependency
- [ ] Create `internal/ui/views/golden_test_helpers.go` (init, sendKeys, key helpers)
- [ ] Add `.gitattributes` entry: `*.golden -text`
- [ ] Add Makefile targets: `test-golden`, `test-golden-update`, `test-golden-ci`

### Phase 2: View Snapshots (start with 2-3 views)
- [ ] Instances view: loaded, loading, error, empty, action menu
- [ ] Instance details view: loaded with tabs
- [ ] Projects view: loaded, loading

### Phase 3: Dialog & Overlay Snapshots
- [ ] Confirm dialog (type-to-confirm delete)
- [ ] Command palette
- [ ] Project selector modal
- [ ] Action menu overlays

### Phase 4: App-Level Snapshots
- [ ] Full layout with sidebar + content
- [ ] Sidebar collapsed vs expanded

### Phase 5: Expand Coverage
- [ ] Remaining list views (buckets, disks, snapshots, images, networks, etc.)
- [ ] Form views (instance create, bucket create)
- [ ] Detail views with tabs
- [ ] Error states across views

## What NOT to Snapshot
- Observability charts with live timestamps (non-deterministic)
- Every sort/filter permutation (cover default only)
- Every scroll position (top position only)
- Target ~4-6 golden files per view: loaded, loading, error, empty, 1-2 dialogs
