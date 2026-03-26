# Golden File Visual Regression Tests — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add golden file snapshot testing to catch visual regressions in TUI rendering — broken layouts, missing content, style changes.

**Architecture:** Use `github.com/charmbracelet/x/exp/golden` to compare `View()` output (with full ANSI escape codes) against committed `.golden` files. Views are constructed directly in tests, fixture data is injected via internal message types. Force `termenv.TrueColor` for deterministic color output.

**Tech Stack:** `charmbracelet/x/exp/golden`, `termenv`, existing `testify/assert`, Go's `testing` package.

---

### Task 1: Add Dependencies and Git Configuration

**Files:**
- Modify: `go.mod`
- Create: `.gitattributes`
- Modify: `Makefile`

**Step 1: Add the golden package dependency**

Run:
```bash
cd /Users/vlad/dev/my/gcon && go get github.com/charmbracelet/x/exp/golden@latest
```
Expected: go.mod updated with `charmbracelet/x` dependency.

**Step 2: Create .gitattributes to prevent golden file corruption**

Create `.gitattributes` at project root:
```
*.golden -text
```

**Step 3: Add Makefile targets**

Add after the `test-coverage` target (line 53 in Makefile):

```makefile
# Run golden snapshot tests
test-golden:
	$(GOTEST) -v ./internal/ui/... -run Golden

# Regenerate golden files after intentional rendering changes
test-golden-update:
	$(GOTEST) ./internal/ui/... -run Golden -update

```

Also update the `help` target to include the new targets:
```
@echo "  test-golden  - Run golden snapshot tests"
@echo "  test-golden-update - Regenerate golden files"
```

**Step 4: Run go mod tidy**

Run:
```bash
cd /Users/vlad/dev/my/gcon && go mod tidy
```
Expected: go.sum updated, no errors.

**Step 5: Commit**

```bash
git add go.mod go.sum .gitattributes Makefile
git commit -m "2026-03-26: add golden test infrastructure (dep, gitattributes, makefile)"
```

---

### Task 2: Create Golden Test Helpers

**Files:**
- Create: `internal/ui/views/golden_helpers_test.go`

**Step 1: Create the test helper file**

```go
package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/context"
)

func init() {
	// Force TrueColor so ANSI output is identical regardless of CI terminal
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// Standard terminal size for all golden tests
const (
	goldenWidth  = 120
	goldenHeight = 40
)

// goldenContext returns a ProgramContext sized for golden tests.
func goldenContext() *context.ProgramContext {
	ctx := context.New()
	ctx.ContentWidth = goldenWidth
	ctx.ContentHeight = goldenHeight
	ctx.ScreenWidth = goldenWidth
	ctx.ScreenHeight = goldenHeight
	ctx.ProjectID = "test-project"
	return ctx
}

// sendKey simulates a single rune key press on a view.
func sendKey(view View, r rune) tea.Cmd {
	return view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
}

// sendSpecialKey simulates a special key press (Enter, Esc, Tab, etc.).
func sendSpecialKey(view View, k tea.KeyType) tea.Cmd {
	return view.Update(tea.KeyMsg{Type: k})
}

// freezeSpinner advances the spinner to a known frame for deterministic output.
// Call once after creating a view in loading state.
func freezeSpinner(view View) {
	// Tick the spinner 3 times to land on a stable frame
	for range 3 {
		view.Update(spinnerTickMsg())
	}
}

// spinnerTickMsg creates a spinner tick message.
// The spinner.TickMsg type requires an ID and Tag which are internal,
// so we use the spinner's own Tick command pattern.
// Instead, we just render the view — the spinner frame is whatever it was at init.
// This function is a placeholder; views may need their own freeze logic.
func spinnerTickMsg() tea.Msg {
	return nil
}

// --- Test Fixture Data ---

func testInstances() []gcp.Instance {
	return []gcp.Instance{
		{
			Name:        "web-server-1",
			Zone:        "us-central1-a",
			MachineType: "e2-medium",
			Status:      "RUNNING",
			InternalIP:  "10.128.0.2",
			ExternalIP:  "35.192.0.1",
			CreatedAt:   "2026-01-15T10:30:00Z",
		},
		{
			Name:        "db-server-1",
			Zone:        "us-central1-b",
			MachineType: "n2-standard-4",
			Status:      "RUNNING",
			InternalIP:  "10.128.0.3",
			ExternalIP:  "",
			CreatedAt:   "2026-01-10T08:00:00Z",
		},
		{
			Name:        "dev-instance",
			Zone:        "us-east1-b",
			MachineType: "e2-micro",
			Status:      "TERMINATED",
			InternalIP:  "10.142.0.5",
			ExternalIP:  "",
			CreatedAt:   "2026-02-20T14:00:00Z",
		},
		{
			Name:        "staging-api",
			Zone:        "europe-west1-b",
			MachineType: "e2-small",
			Status:      "SUSPENDED",
			InternalIP:  "10.132.0.8",
			ExternalIP:  "34.76.0.12",
			CreatedAt:   "2026-03-01T09:00:00Z",
		},
	}
}

func testProjects() []gcp.Project {
	return []gcp.Project{
		{ID: "my-project-prod", Name: "My Project (Prod)", Number: 123456, State: "ACTIVE"},
		{ID: "my-project-staging", Name: "My Project (Staging)", Number: 789012, State: "ACTIVE"},
		{ID: "old-project", Name: "Old Project", Number: 345678, State: "DELETE_REQUESTED"},
	}
}
```

**Step 2: Verify the file compiles**

Run:
```bash
cd /Users/vlad/dev/my/gcon && go build ./internal/ui/views/
```
Expected: No errors. If `termenv` import path is wrong, fix it — it may be `github.com/muesli/termenv` or re-exported via lipgloss.

**Step 3: Commit**

```bash
git add internal/ui/views/golden_helpers_test.go
git commit -m "2026-03-26: add golden test helpers and fixture data"
```

---

### Task 3: Projects View Golden Tests

**Files:**
- Create: `internal/ui/views/projects_golden_test.go`

This is the simplest view — start here to validate the golden file workflow end-to-end.

**Step 1: Write the golden test file**

```go
package views

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
)

func TestGolden_ProjectsView_Loaded(t *testing.T) {
	view := NewProjectsView(nil)
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(projectsLoadedMsg{projects: testProjects()})

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_ProjectsView_Loading(t *testing.T) {
	view := NewProjectsView(nil)
	ctx := goldenContext()
	view.SetContext(ctx)
	// View starts in loading state by default — don't send loaded message

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_ProjectsView_Error(t *testing.T) {
	view := NewProjectsView(nil)
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(projectsErrorMsg{err: fmt.Errorf("permission denied: caller does not have resourcemanager.projects.list")})

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_ProjectsView_Empty(t *testing.T) {
	view := NewProjectsView(nil)
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(projectsLoadedMsg{projects: nil})

	golden.RequireEqual(t, []byte(view.View()))
}
```

**Step 2: Generate golden files**

Run:
```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/ui/views/ -run "TestGolden_ProjectsView" -update -v
```
Expected: 4 golden files created in `internal/ui/views/testdata/`.

**Step 3: Verify golden files exist and contain ANSI codes**

Run:
```bash
ls -la internal/ui/views/testdata/TestGolden_ProjectsView_*.golden
cat -v internal/ui/views/testdata/TestGolden_ProjectsView_Loaded.golden | head -5
```
Expected: Files exist. `cat -v` shows `^[[` escape sequences (ANSI codes).

**Step 4: Run tests without -update to verify they pass**

Run:
```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/ui/views/ -run "TestGolden_ProjectsView" -v
```
Expected: All 4 tests pass.

**Step 5: Commit**

```bash
git add internal/ui/views/projects_golden_test.go internal/ui/views/testdata/
git commit -m "2026-03-26: add golden tests for projects view"
```

---

### Task 4: Instances View Golden Tests

**Files:**
- Create: `internal/ui/views/instances_golden_test.go`

This view has more states: loaded, loading, error, empty, action menu, confirm dialogs.

**Step 1: Write the golden test file**

```go
package views

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/slayer/gcon/internal/gcp"
)

func TestGolden_InstancesView_Loaded(t *testing.T) {
	view := NewInstancesView("test-project")
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(instancesLoadedMsg{instances: testInstances()})

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_InstancesView_Loading(t *testing.T) {
	view := NewInstancesView("test-project")
	ctx := goldenContext()
	view.SetContext(ctx)
	// Simulate client ready so we get "Loading instances..." not "Initializing..."
	view.Update(computeClientReadyMsg{client: nil})

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_InstancesView_Error(t *testing.T) {
	view := NewInstancesView("test-project")
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(instancesErrorMsg{err: fmt.Errorf("compute.instances.list: permission denied")})

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_InstancesView_Empty(t *testing.T) {
	view := NewInstancesView("test-project")
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(instancesLoadedMsg{instances: nil})

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_InstancesView_ActionMenu(t *testing.T) {
	view := NewInstancesView("test-project")
	ctx := goldenContext()
	view.SetContext(ctx)
	view.Update(instancesLoadedMsg{instances: testInstances()})
	// Open action menu on first (selected) instance
	sendKey(view, '.')

	golden.RequireEqual(t, []byte(view.View()))
}

func TestGolden_InstancesView_DeleteConfirm(t *testing.T) {
	view := NewInstancesView("test-project")
	ctx := goldenContext()
	view.SetContext(ctx)
	instances := testInstances()
	view.Update(instancesLoadedMsg{instances: instances})
	// Trigger delete — this may require instance details to be fetched first.
	// If D key triggers an async fetch, we need to inject the details message instead.
	// The view opens delete confirm after receiving instanceDeleteDetailsMsg.
	view.Update(instanceDeleteDetailsMsg{
		instance: &instances[0],
		details:  &gcp.InstanceDetails{Name: instances[0].Name, Zone: instances[0].Zone},
	})

	golden.RequireEqual(t, []byte(view.View()))
}
```

**Step 2: Generate golden files**

Run:
```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/ui/views/ -run "TestGolden_InstancesView" -update -v
```
Expected: Golden files created. If any test panics (nil pointer on computeClient etc.), fix the test setup — views should render without a live client when data is injected directly.

**Step 3: Verify tests pass**

Run:
```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/ui/views/ -run "TestGolden_InstancesView" -v
```
Expected: All tests pass.

**Step 4: Commit**

```bash
git add internal/ui/views/instances_golden_test.go internal/ui/views/testdata/
git commit -m "2026-03-26: add golden tests for instances view"
```

---

### Task 5: App-Level Golden Tests

**Files:**
- Create: `internal/ui/app_golden_test.go`

Tests the full layout: header + sidebar + content + footer.

**Step 1: Write the golden test file**

```go
package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/muesli/termenv"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/views"
)

func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

const (
	goldenAppWidth  = 120
	goldenAppHeight = 40
)

func TestGolden_App_ProjectsView(t *testing.T) {
	app := createTestApp()
	// Trigger a window size message so layout calculates
	app.Update(tea.WindowSizeMsg{Width: goldenAppWidth, Height: goldenAppHeight})

	golden.RequireEqual(t, []byte(app.View()))
}

func TestGolden_App_InstancesWithSidebar(t *testing.T) {
	app := createTestApp()
	app.Update(tea.WindowSizeMsg{Width: goldenAppWidth, Height: goldenAppHeight})
	simulateProjectSelection(app)
	// Inject instances data — need to check how app routes this message
	// The app creates the instancesView and routes instancesLoadedMsg to it.
	// We may need to create the view manually and inject data.
	if app.instancesView == nil {
		app.instancesView = views.NewInstancesView("test-project")
		app.instancesView.SetContext(app.ctx)
	}
	// Send loaded message through the app so it routes correctly
	app.Update(views.InstancesLoadedMsg{Instances: testAppInstances()})

	golden.RequireEqual(t, []byte(app.View()))
}
```

**Important note for implementer:** The app routes messages through its own `Update()`. Internal view messages like `instancesLoadedMsg` (lowercase) can't be sent from outside the package. The implementer needs to check:
1. Whether `instancesLoadedMsg` is exported or not
2. If not, set up the view state directly: create the view, call `SetContext`, and set `view.loading = false` + `view.instances = data` if those fields are accessible

If direct field access isn't possible because they're unexported, the approach is:
- Create view
- Use the exported `Update()` method but you'll need to create the message from within the `views` package — meaning the app-level golden tests might need to be in the `views_test` package (external test package) using only exported APIs, OR use a test helper inside the `views` package.

The implementer should adapt based on what's actually exported. The goal is: app renders with sidebar + populated content.

**Step 2: Generate and verify**

Run:
```bash
cd /Users/vlad/dev/my/gcon && go test ./internal/ui/ -run "TestGolden_App" -update -v
```

**Step 3: Commit**

```bash
git add internal/ui/app_golden_test.go internal/ui/testdata/
git commit -m "2026-03-26: add app-level golden tests"
```

---

### Task 6: Run Full Suite, Lint, and Final Commit

**Step 1: Run all tests**

Run:
```bash
cd /Users/vlad/dev/my/gcon && make test
```
Expected: All existing tests + new golden tests pass.

**Step 2: Run linter**

Run:
```bash
cd /Users/vlad/dev/my/gcon && make lint
```
Expected: No new lint errors from golden test files.

**Step 3: Run golden tests specifically**

Run:
```bash
cd /Users/vlad/dev/my/gcon && make test-golden
```
Expected: All golden tests pass, validating the Makefile target works.

**Step 4: Verify -update workflow**

Run:
```bash
cd /Users/vlad/dev/my/gcon && make test-golden-update && make test-golden
```
Expected: Update regenerates files, subsequent run passes.

---

## Notes for Implementer

### Spinner Determinism
The `spinner.TickMsg` type has internal fields (`ID`, `tag`). You cannot construct one manually. Options:
1. Accept that the spinner character varies and don't golden-test loading states
2. Replace the spinner output with a fixed string in the test (e.g., `strings.Replace` before comparison)
3. Access `view.spinner.Tick` to get a valid tick command, execute it, use the resulting message

Option 1 is simplest for the initial implementation. Loading states are simple enough that a `strings.Contains` unit test is sufficient. Focus golden tests on the *loaded* states where layout matters most.

### Package Boundaries
View message types like `instancesLoadedMsg` are unexported (lowercase). Golden tests in `internal/ui/views/` (same package) can use them directly. App-level tests in `internal/ui/` cannot. For app-level tests, either:
- Expose a test helper in the views package
- Set view state via exported methods only
- Skip app-level golden tests initially and add them later

### ANSI Color Determinism
The `init()` with `lipgloss.SetColorProfile(termenv.TrueColor)` is **per-package**. Both `views` and `ui` test packages need their own `init()` if they have golden tests.

### What to Snapshot First
Priority order:
1. Instances view (most used, has dialogs)
2. Projects view (simplest, validates workflow)
3. App layout (catches sidebar/header bugs)
4. Everything else can be added incrementally

### Future Expansion
After these initial tests work, add golden tests to other views following the same pattern. Each new view needs ~4 tests: loaded, error, empty, one dialog. This is mechanical work that can be done view-by-view.
