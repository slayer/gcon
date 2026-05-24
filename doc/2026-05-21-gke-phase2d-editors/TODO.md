# GKE Phase 2d Implementation Plan

**Goal:** Land the editors Phase 2c MVP deferred — cluster resource labels, pool k8s labels + taints, cluster recurring maintenance windows. Diff preview for all new entries (added / removed / changed).

**Reference docs:**
- Spec: `doc/2026-05-21-gke-phase2d-editors/Design.md`
- Closest analogs: `internal/ui/views/instance_editor.go` (labeledit-as-sub-state pattern), `internal/ui/components/labeledit/` (component to extend), `internal/ui/views/gke_cluster_edit.go` (3-state machine to extend with editor sub-states)

**Pattern reminders:**
- Editors are sub-states of the existing edit views, NOT new top-level views. View() branches on state; Update() routes to active editor.
- `HasTextInputFocused()` must delegate to the active editor when in `*EditingLabels` / `*EditingTaints` state.
- Wrap maintenance recurring fields with a baseline-defaulting comparison in `computeEdit` so unrecognized RRULEs don't false-positive (mirror Phase 2c's logging-baseline fix).

---

## File structure

**New files:**
- `internal/ui/components/taintedit/taintedit.go` — node-pool taint editor
- `internal/ui/components/taintedit/taintedit_test.go`

**Modified files:**
- `internal/ui/components/labeledit/labeledit.go` — pluggable validators (k8s rules)
- `internal/ui/components/labeledit/labeledit_test.go` — k8s-mode tests
- `internal/gcp/gke_edit.go` — `MaintenanceWindow` gains Days/Start/Duration; `MaintenanceKindRecurring` const; `buildSetMaintenancePolicyRequest` handles recurring
- `internal/gcp/gke_edit_test.go` — recurring window tests
- `internal/gcp/gke.go` — `ClusterDetails.MaintenanceRecurring` projection
- `internal/gcp/gke_test.go` — projection tests
- `internal/ui/views/gke_cluster_edit.go` — labels editor sub-state + recurring maintenance form fields + diff additions
- `internal/ui/views/gke_cluster_edit_test.go` — coverage
- `internal/ui/views/gke_node_pool_edit.go` — labels + taints editor sub-states + diff additions
- `internal/ui/views/gke_node_pool_edit_test.go`
- `.claude/rules/key-bindings.md` — document the new sub-editor keys
- `CLAUDE.md`, `README.md`

---

## Task 1: labeledit — pluggable validators

**Files:** `internal/ui/components/labeledit/labeledit.go` + test.

Today labeledit has package-level `keyPattern` and `valuePattern` regexes baked in. Make them configurable per-Editor.

- [ ] **Step 1: Add option type + setter, keep current defaults**

```go
// In labeledit.go (new exported type)
type Validators struct {
    KeyPattern   *regexp.Regexp
    ValuePattern *regexp.Regexp
    KeyError     string // shown when key fails validation
    ValueError   string // shown when value fails validation
}

// SetValidators overrides the GCP-label defaults. Pass nil patterns to
// disable validation entirely (the taint editor uses this for empty values).
func (e *Editor) SetValidators(v Validators) {
    e.validators = v
}
```

Replace the package-level regex uses inside `submitEdit` / row-render validation with `e.validators.KeyPattern` etc. Default Editor (no SetValidators call) keeps current behaviour.

- [ ] **Step 2: Tests**

```go
func TestLabelEditor_K8sLabelKeyAcceptsDotsAndSlashes(t *testing.T) {
    e := New(map[string]string{})
    e.SetValidators(Validators{
        KeyPattern:   regexp.MustCompile(`^([a-z0-9]([-a-z0-9.]*[a-z0-9])?/)?[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`),
        ValuePattern: regexp.MustCompile(`^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$|^$`),
    })
    // Simulate adding a label with a DNS-style key.
    // (Use the editor's public Add / Edit flow; for unit-level, validate the
    // pattern directly works on "kubernetes.io/role".)
    assert.True(t, e.validators.KeyPattern.MatchString("kubernetes.io/role"))
    assert.True(t, e.validators.KeyPattern.MatchString("simple-key"))
    assert.False(t, e.validators.KeyPattern.MatchString("KEY UPPER NOT ALLOWED")) // spaces fail
}
```

- [ ] **Step 3: Commit**

```bash
go test ./internal/ui/components/labeledit/ -v
make lint
git add internal/ui/components/labeledit/
git commit -m "2026-05-21: GKE phase 2d — labeledit pluggable validators"
```

---

## Task 2: taintedit component

**Files:** Create `internal/ui/components/taintedit/{taintedit.go, taintedit_test.go}`.

Mirror labeledit's shape — same row navigation, add/edit/delete keys, edit mode with text inputs. The "row" has three fields: key (text), value (text, may be empty), effect (dropdown over `NO_SCHEDULE / PREFER_NO_SCHEDULE / NO_EXECUTE`).

- [ ] **Step 1: Failing tests**

```go
package taintedit

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/slayer/gcon/internal/gcp"
)

func TestNew_PopulatesFromInitial(t *testing.T) {
    initial := []gcp.NodeTaint{
        {Key: "dedicated", Value: "gpu", Effect: "NO_SCHEDULE"},
    }
    e := New(initial)
    got := e.GetTaints()
    require.Len(t, got, 1)
    assert.Equal(t, "dedicated", got[0].Key)
    assert.Equal(t, "gpu", got[0].Value)
    assert.Equal(t, "NO_SCHEDULE", got[0].Effect)
    assert.False(t, e.IsDirty(), "fresh editor must not be dirty")
}

func TestEditor_IsDirty_AfterAdd(t *testing.T) {
    // Add a taint via the public flow (simulated). For unit-level it's
    // acceptable to directly populate via a test helper. If the editor only
    // exposes Update(KeyMsg), drive it with the same keystrokes the user
    // would: a, type "k", Tab, type "v", Tab, Enter to confirm.
}

func TestEditor_GetTaints_ReturnsCopy(t *testing.T) {
    initial := []gcp.NodeTaint{{Key: "k", Value: "v", Effect: "NO_SCHEDULE"}}
    e := New(initial)
    got := e.GetTaints()
    got[0].Key = "mutated"
    second := e.GetTaints()
    assert.Equal(t, "k", second[0].Key, "GetTaints must return a copy")
}
```

- [ ] **Step 2: Implement Editor**

Skeleton (full implementation will be ~300 LOC, mirror labeledit):

```go
package taintedit

import (
    "regexp"
    "sort"
    "strings"
    "github.com/charmbracelet/bubbles/key"
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/slayer/gcon/internal/gcp"
)

// Effect values supported by GCP.
const (
    EffectNoSchedule       = "NO_SCHEDULE"
    EffectPreferNoSchedule = "PREFER_NO_SCHEDULE"
    EffectNoExecute        = "NO_EXECUTE"
)

var effectCycle = []string{EffectNoSchedule, EffectPreferNoSchedule, EffectNoExecute}

type Editor struct {
    taints      []gcp.NodeTaint
    original    []gcp.NodeTaint
    cursor      int
    editing     bool
    editIdx     int
    editFocus   int           // 0=key, 1=value, 2=effect
    keyInput    textinput.Model
    valueInput  textinput.Model
    editEffect  string
    width       int
    height      int
    keyMap      keyMap
    styles      styles
    addPending  bool
}

func New(initial []gcp.NodeTaint) *Editor {
    // copy initial → e.taints and e.original
    // init textinputs
}

func (e *Editor) GetTaints() []gcp.NodeTaint {
    out := make([]gcp.NodeTaint, len(e.taints))
    copy(out, e.taints)
    return out
}

func (e *Editor) IsDirty() bool {
    if len(e.taints) != len(e.original) { return true }
    // compare element-by-element by sorted key
    a := append([]gcp.NodeTaint(nil), e.taints...)
    b := append([]gcp.NodeTaint(nil), e.original...)
    sort.Slice(a, func(i, j int) bool { return a[i].Key < a[j].Key })
    sort.Slice(b, func(i, j int) bool { return b[i].Key < b[j].Key })
    for i := range a {
        if a[i] != b[i] { return true }
    }
    return false
}

func (e *Editor) IsEditing() bool { return e.editing }

func (e *Editor) HasTextInputFocused() bool {
    return e.editing && (e.editFocus == 0 || e.editFocus == 1)
}

func (e *Editor) Update(msg tea.Msg) tea.Cmd {
    // route key by mode (navigation vs edit)
    // - navigation: ↑/↓/j/k cursor; a add; e/Enter edit; x/Del delete; Ctrl+S save; Esc cancel
    // - edit: Tab cycles key→value→effect; Space toggles effect when focused; Enter confirms; Esc cancel
}

// SaveRequestedMsg mirrors labeledit's pattern — parent listens for this msg.
type SaveRequestedMsg struct{}
type CancelRequestedMsg struct{}

func (e *Editor) View() string {
    // header + row list + footer help
    // each row: highlighted on cursor; in edit mode show the three inputs
}
```

Validators: key uses the k8s name regex (same as labeledit k8s mode); value can be empty. Effect is always one of the three constants (enforced by the cycle).

- [ ] **Step 3: Run + lint + commit**

```bash
go test ./internal/ui/components/taintedit/ -v
make lint
git add internal/ui/components/taintedit/
git commit -m "2026-05-21: GKE phase 2d — taintedit component (k8s node-pool taints)"
```

---

## Task 3: MaintenanceWindow recurring support

**Files:** `internal/gcp/gke_edit.go` + test.

- [ ] **Step 1: Tests**

```go
func TestBuildSetMaintenancePolicyRequest_Recurring(t *testing.T) {
    req := buildSetMaintenancePolicyRequest(MaintenanceWindow{
        Kind:     MaintenanceKindRecurring,
        Days:     []string{"MO", "WE", "FR"},
        Start:    "03:00",
        Duration: "4h",
    })
    require.NotNil(t, req.MaintenancePolicy.Window.RecurringWindow)
    rw := req.MaintenancePolicy.Window.RecurringWindow
    assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO,WE,FR", rw.Recurrence)
    // Start/end times: built from a fixed baseline date (2026-01-04 = Sunday).
    assert.Contains(t, rw.Window.StartTime, "T03:00:00")
    assert.Contains(t, rw.Window.EndTime, "T07:00:00")
}

func TestBuildSetMaintenancePolicyRequest_RecurringSingleDay(t *testing.T) {
    req := buildSetMaintenancePolicyRequest(MaintenanceWindow{
        Kind:     MaintenanceKindRecurring,
        Days:     []string{"SU"},
        Start:    "02:00",
        Duration: "1h",
    })
    rw := req.MaintenancePolicy.Window.RecurringWindow
    assert.Equal(t, "FREQ=WEEKLY;BYDAY=SU", rw.Recurrence)
}
```

- [ ] **Step 2: Implement**

Add const + fields to `MaintenanceWindow`:

```go
const MaintenanceKindRecurring MaintenanceKind = "recurring"

type MaintenanceWindow struct {
    Kind     MaintenanceKind
    Daily    string

    // Used when Kind == MaintenanceKindRecurring:
    Days     []string // subset of {"MO","TU","WE","TH","FR","SA","SU"}
    Start    string   // "HH:MM" UTC
    Duration string   // "Nh", 1-23
}
```

Extend `buildSetMaintenancePolicyRequest`:

```go
case MaintenanceKindRecurring:
    // Anchor baseline: 2026-01-04T00:00:00Z is a Sunday. RRULE recurrence
    // makes the actual date irrelevant — only the time-of-day matters.
    start := composeRFC3339(mw.Start, 0)
    end := composeRFC3339(mw.Start, parseHoursOr(mw.Duration, 4))
    policy.Window = &container.MaintenanceWindow{
        RecurringWindow: &container.RecurringTimeWindow{
            Window: &container.TimeWindow{StartTime: start, EndTime: end},
            Recurrence: "FREQ=WEEKLY;BYDAY=" + strings.Join(mw.Days, ","),
        },
    }
```

Helpers:

```go
func composeRFC3339(timeOfDay string, addHours int) string {
    // baseline := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)
    // parse "HH:MM" → hour, minute
    // return baseline.Add(...).Format(time.RFC3339)
}

func parseHoursOr(s string, fallback int) int {
    // strip "h" suffix, parse int, clamp 1-23, fallback if invalid
}
```

- [ ] **Step 3: Run + lint + commit**

```bash
go test ./internal/gcp/ -run TestBuildSetMaintenance -v
make lint
git add internal/gcp/gke_edit.go internal/gcp/gke_edit_test.go
git commit -m "2026-05-21: GKE phase 2d — MaintenanceWindow recurring (days + start + duration)"
```

---

## Task 4: `ClusterDetails.MaintenanceRecurring` projection

**Files:** `internal/gcp/gke.go` + `internal/gcp/gke_test.go`.

The form needs to pre-populate when an existing cluster already has a recurring window. Parse the RRULE on read.

- [ ] **Step 1: Tests**

```go
func TestConvertCluster_RecurringWindow(t *testing.T) {
    raw := &container.Cluster{
        Name: "prod",
        MaintenancePolicy: &container.MaintenancePolicy{
            Window: &container.MaintenanceWindow{
                RecurringWindow: &container.RecurringTimeWindow{
                    Window: &container.TimeWindow{
                        StartTime: "2026-01-05T03:00:00Z",
                        EndTime:   "2026-01-05T07:00:00Z",
                    },
                    Recurrence: "FREQ=WEEKLY;BYDAY=MO,WE,FR",
                },
            },
        },
    }
    details := buildTestClusterDetails(raw)
    require.NotNil(t, details.MaintenanceRecurring)
    assert.Equal(t, []string{"MO", "WE", "FR"}, details.MaintenanceRecurring.Days)
    assert.Equal(t, "03:00", details.MaintenanceRecurring.Start)
    assert.Equal(t, "4h", details.MaintenanceRecurring.Duration)
}

func TestConvertCluster_RecurringUnsupportedRRULE(t *testing.T) {
    // RRULE we don't parse → projection stays nil; UI will show placeholder.
    raw := &container.Cluster{
        MaintenancePolicy: &container.MaintenancePolicy{
            Window: &container.MaintenanceWindow{
                RecurringWindow: &container.RecurringTimeWindow{
                    Window: &container.TimeWindow{StartTime: "2026-01-05T03:00:00Z", EndTime: "2026-01-05T07:00:00Z"},
                    Recurrence: "FREQ=MONTHLY;BYMONTHDAY=1",
                },
            },
        },
    }
    details := buildTestClusterDetails(raw)
    assert.Nil(t, details.MaintenanceRecurring)
}
```

- [ ] **Step 2: Implement projection**

```go
type RecurringWindow struct {
    Days     []string
    Start    string
    Duration string
}

type ClusterDetails struct {
    // ... existing ...
    MaintenanceRecurring *RecurringWindow // nil when no recurring window OR unrecognized RRULE
}

// In GetCluster (after MaintenanceDaily extraction):
if rw := maintenanceRecurringPolicy(c); rw != nil {
    details.MaintenanceRecurring = rw
}

func maintenanceRecurringPolicy(c *container.Cluster) *RecurringWindow {
    if c.MaintenancePolicy == nil || c.MaintenancePolicy.Window == nil || c.MaintenancePolicy.Window.RecurringWindow == nil {
        return nil
    }
    rw := c.MaintenancePolicy.Window.RecurringWindow
    days := parseWeeklyByday(rw.Recurrence) // returns nil if RRULE not parseable
    if len(days) == 0 {
        return nil
    }
    start, dur := parseRecurringWindowTimes(rw.Window) // "HH:MM" + "Nh"
    if start == "" {
        return nil
    }
    return &RecurringWindow{Days: days, Start: start, Duration: dur}
}

func parseWeeklyByday(rrule string) []string {
    // Only handles "FREQ=WEEKLY;BYDAY=X,Y,Z" (in any order of fields).
    // Returns nil for anything else.
}

func parseRecurringWindowTimes(w *container.TimeWindow) (start, duration string) {
    // Parse RFC3339, extract HH:MM and (end-start) → "Nh".
}
```

- [ ] **Step 3: Run + lint + commit**

```bash
go test ./internal/gcp/ -run 'TestConvertCluster_Recurring' -v
make lint
git add internal/gcp/gke.go internal/gcp/gke_test.go
git commit -m "2026-05-21: GKE phase 2d — ClusterDetails.MaintenanceRecurring projection"
```

---

## Task 5: Cluster edit — labels editor sub-state

**Files:** `internal/ui/views/gke_cluster_edit.go` + test.

- [ ] **Step 1: Add new state + field**

```go
type clusterEditState int

const (
    clusterEditStateForm clusterEditState = iota
    clusterEditStateEditingLabels   // NEW
    clusterEditStateDiff
    clusterEditStateSaving
)

type GKEClusterEditView struct {
    // ... existing ...
    labelEditor    *labeledit.Editor
    editingLabels  map[string]string  // last-saved snapshot from the editor
}
```

- [ ] **Step 2: Open the editor**

When the user is focused on the resource labels field and presses Enter (or `l`):

```go
func (v *GKEClusterEditView) openLabelEditor() tea.Cmd {
    initial := v.editingLabels
    if initial == nil {
        // First time opening — clone from details.
        initial = make(map[string]string, len(v.details.ResourceLabels))
        for k, val := range v.details.ResourceLabels {
            initial[k] = val
        }
    }
    v.labelEditor = labeledit.New(initial)
    v.labelEditor.SetSize(v.width-4, v.height-8)
    // Default validators (GCP-label rules) — no SetValidators call needed.
    v.state = clusterEditStateEditingLabels
    return nil
}
```

- [ ] **Step 3: Route messages to editor when active**

```go
func (v *GKEClusterEditView) Update(msg tea.Msg) tea.Cmd {
    if v.state == clusterEditStateEditingLabels && v.labelEditor != nil {
        switch m := msg.(type) {
        case labeledit.SaveRequestedMsg:
            v.editingLabels = v.labelEditor.GetLabels()
            v.labelEditor = nil
            v.state = clusterEditStateForm
            return nil
        case labeledit.CancelRequestedMsg:
            v.labelEditor = nil
            v.state = clusterEditStateForm
            return nil
        case tea.KeyMsg:
            return v.labelEditor.Update(m)
        }
        return v.labelEditor.Update(msg)
    }
    // ... existing dispatch ...
}

func (v *GKEClusterEditView) View() string {
    if v.state == clusterEditStateEditingLabels && v.labelEditor != nil {
        return v.labelEditor.View()
    }
    // ... existing ...
}

func (v *GKEClusterEditView) HasTextInputFocused() bool {
    if v.state == clusterEditStateEditingLabels && v.labelEditor != nil {
        return v.labelEditor.HasTextInputFocused()
    }
    // ... existing ...
}
```

- [ ] **Step 4: Wire into computeEdit**

```go
// In computeEdit:
initialLabels := v.details.ResourceLabels
newLabels := v.editingLabels // nil if user never opened the editor → no change
if newLabels != nil && !mapsEqual(initialLabels, newLabels) {
    if basic == nil { basic = &gcp.ClusterEdit{} }
    cloned := make(map[string]string, len(newLabels))
    for k, val := range newLabels { cloned[k] = val }
    basic.ResourceLabels = &cloned
    basic.ResourceLabelsFingerprint = v.details.ResourceLabelsFingerprint
}
```

`mapsEqual` is a tiny helper — likely already exists; if not, add it.

- [ ] **Step 5: Diff rendering** — extend `renderDiff` to show added / removed / changed label entries.

- [ ] **Step 6: Form interaction** — when the resource_labels read-only field is focused, intercept Enter / `l` keys to open the editor. The form framework's read-only fields don't normally accept key events, so this happens in the edit view's `handleKeyMsg`:

```go
if v.state == clusterEditStateForm && v.Form.FocusedFieldID() == "resource_labels" {
    if msg.Type == tea.KeyEnter || msg.String() == "l" {
        return v.openLabelEditor()
    }
}
```

Confirm `Form.FocusedFieldID()` exists — if not, fall back to a key handler that's always active when in form state.

- [ ] **Step 7: Tests**

```go
func TestGKEClusterEdit_LabelsEditedTransitionsToDiff(t *testing.T) {
    details := &gcp.ClusterDetails{
        Cluster:           gcp.Cluster{Name: "prod"},
        LoggingService:    "logging.googleapis.com/kubernetes",
        MonitoringService: "monitoring.googleapis.com/kubernetes",
        ResourceLabels:    map[string]string{"env": "dev"},
        ResourceLabelsFingerprint: "fp1",
    }
    v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
    // Simulate user editing labels — directly populate v.editingLabels for unit
    // test (the editor's Update flow is covered separately).
    v.editingLabels = map[string]string{"env": "prod"} // changed
    cmd := v.handleSubmit()
    assert.Nil(t, cmd)
    assert.Equal(t, clusterEditStateDiff, v.state)
    require.NotNil(t, v.pendingBasic)
    require.NotNil(t, v.pendingBasic.ResourceLabels)
    assert.Equal(t, "prod", (*v.pendingBasic.ResourceLabels)["env"])
    assert.Equal(t, "fp1", v.pendingBasic.ResourceLabelsFingerprint)
}

func TestGKEClusterEdit_LabelsUnchangedNoDiff(t *testing.T) {
    details := &gcp.ClusterDetails{ /* same labels */ }
    v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
    v.editingLabels = map[string]string{"env": "dev"} // identical to initial
    cmd := v.handleSubmit()
    assert.Nil(t, cmd)
    assert.ErrorIs(t, v.err, errClusterEditNoChanges)
}
```

- [ ] **Step 8: Commit**

```bash
go test ./internal/ui/views -run 'TestGKEClusterEdit' -v
make lint
git add internal/ui/views/gke_cluster_edit.go internal/ui/views/gke_cluster_edit_test.go
git commit -m "2026-05-21: GKE phase 2d — cluster edit: labels editor sub-state"
```

---

## Task 6: Cluster edit — recurring maintenance UI

**Files:** `internal/ui/views/gke_cluster_edit.go` + test.

Extend the Maintenance section with three new fields, conditionally visible when `maintenance_kind == "recurring"`.

- [ ] **Step 1: Extend buildForm**

Add a third option to the kind dropdown + three new fields:

```go
AddField(forms.NewDropdownField("maintenance_kind", "Maintenance Window").
    SetOptions([]forms.Option{
        {Value: "none", Label: "None (clear)"},
        {Value: "daily", Label: "Daily window"},
        {Value: "recurring", Label: "Recurring (weekly)"},
    }))

AddField(forms.NewMultiSelectField("maintenance_days", "Days").
    SetOptions([]forms.Option{
        {Value: "MO", Label: "Mon"}, {Value: "TU", Label: "Tue"},
        {Value: "WE", Label: "Wed"}, {Value: "TH", Label: "Thu"},
        {Value: "FR", Label: "Fri"}, {Value: "SA", Label: "Sat"},
        {Value: "SU", Label: "Sun"},
    }).
    SetHelpText("Days of week the recurring window applies (recurring kind only)"))

AddField(forms.NewTextField("maintenance_recurring_start", "Start Time (UTC)").
    SetPlaceholder("HH:MM").
    SetValidator(validateMaintenanceTime))

AddField(forms.NewNumberField("maintenance_recurring_duration", "Duration (hours)").
    SetValidator(forms.ValidateNumber(1, 23)))
```

Pre-populate from `details.MaintenanceRecurring` when non-nil.

- [ ] **Step 2: Conditional visibility**

`forms.Field` has a `Hidden bool`. After dropdown value changes, hide/show the recurring fields. Two ways:

(a) Re-build the form on every dropdown change (heavy).
(b) Set `Hidden=true` initially on the recurring fields when kind != "recurring", and on each form update, sync visibility from current data.

Take (b). Add a small helper:

```go
func (v *GKEClusterEditView) syncMaintenanceVisibility() {
    kind, _ := v.Form.GetData()["maintenance_kind"].(string)
    if f := v.Form.GetField("maintenance_daily_start"); f != nil {
        f.Hidden = kind != "daily"
    }
    for _, id := range []string{"maintenance_days", "maintenance_recurring_start", "maintenance_recurring_duration"} {
        if f := v.Form.GetField(id); f != nil {
            f.Hidden = kind != "recurring"
        }
    }
}
```

Call it from buildForm (post-SetData) and from Update on every key (or on a forms.FieldChangedMsg if it exists; otherwise the unconditional call on KeyMsg is cheap).

- [ ] **Step 3: computeEdit recurring branch**

```go
case "recurring":
    days := getStringSlice("maintenance_days")
    start := getString("maintenance_recurring_start")
    duration := fmt.Sprintf("%dh", getInt64("maintenance_recurring_duration"))

    // Baseline comparison: when initial kind is "recurring" AND days/start/duration match, no change.
    initial := v.details.MaintenanceRecurring
    initialDays := []string{}
    initialStart, initialDuration := "", ""
    if initial != nil {
        initialDays, initialStart, initialDuration = initial.Days, initial.Start, initial.Duration
    }

    if initialKind == gcp.MaintenanceKindRecurring &&
        slicesEqual(sorted(initialDays), sorted(days)) &&
        initialStart == start && initialDuration == duration {
        // no change
    } else {
        // Validate at least one day picked.
        if len(days) == 0 {
            return nil, nil, errMaintenanceDaysEmpty
        }
        maintenance = &gcp.MaintenanceWindow{
            Kind: gcp.MaintenanceKindRecurring,
            Days: days, Start: start, Duration: duration,
        }
    }
```

`initialKind` comes from existing logic (extend it to detect recurring vs daily vs none based on details).

`errMaintenanceDaysEmpty` is a new sentinel.

- [ ] **Step 4: Test**

```go
func TestGKEClusterEdit_RecurringMaintenanceChangeTransitionsToDiff(t *testing.T) {
    details := &gcp.ClusterDetails{Cluster: gcp.Cluster{Name: "prod"}}
    v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
    v.Form.SetData(map[string]any{
        "maintenance_kind":                "recurring",
        "maintenance_days":                []string{"MO", "WE", "FR"},
        "maintenance_recurring_start":     "03:00",
        "maintenance_recurring_duration":  int64(4),
    })
    cmd := v.handleSubmit()
    assert.Nil(t, cmd)
    assert.Equal(t, clusterEditStateDiff, v.state)
    require.NotNil(t, v.pendingMaintenance)
    assert.Equal(t, gcp.MaintenanceKindRecurring, v.pendingMaintenance.Kind)
    assert.Equal(t, []string{"MO", "WE", "FR"}, v.pendingMaintenance.Days)
    assert.Equal(t, "4h", v.pendingMaintenance.Duration)
}

func TestGKEClusterEdit_RecurringMaintenanceRejectsEmptyDays(t *testing.T) {
    details := &gcp.ClusterDetails{Cluster: gcp.Cluster{Name: "prod"}}
    v := NewGKEClusterEditView("proj", "us-central1", "prod", details)
    v.Form.SetData(map[string]any{
        "maintenance_kind":                "recurring",
        "maintenance_days":                []string{}, // empty!
        "maintenance_recurring_start":     "03:00",
        "maintenance_recurring_duration":  int64(4),
    })
    cmd := v.handleSubmit()
    assert.Nil(t, cmd)
    assert.ErrorIs(t, v.err, errMaintenanceDaysEmpty)
}
```

- [ ] **Step 5: Commit**

```bash
go test ./internal/ui/views -run 'TestGKEClusterEdit' -v
make lint
git add internal/ui/views/gke_cluster_edit.go internal/ui/views/gke_cluster_edit_test.go
git commit -m "2026-05-21: GKE phase 2d — cluster edit: recurring maintenance UI"
```

---

## Task 7: Pool edit — labels editor sub-state (k8s rules)

**Files:** `internal/ui/views/gke_node_pool_edit.go` + test.

Mirror Task 5 but with k8s validators:

```go
v.labelEditor = labeledit.New(initial)
v.labelEditor.SetValidators(labeledit.Validators{
    KeyPattern:   regexp.MustCompile(`^([a-z0-9]([-a-z0-9.]*[a-z0-9])?/)?[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$`),
    ValuePattern: regexp.MustCompile(`^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$|^$`),
    KeyError:     "Invalid k8s label key (optional DNS prefix + name)",
    ValueError:   "Invalid k8s label value (may be empty)",
})
v.labelEditor.SetSize(v.width-4, v.height-8)
v.state = nodePoolEditStateEditingLabels
```

`computeEdit` populates `NodePoolEdit.Labels` when the map differs.

- [ ] **Steps 1-7 same shape as Task 5** (state, open, route, computeEdit wiring, diff render, form key handler, tests, commit).

```bash
git commit -m "2026-05-21: GKE phase 2d — pool edit: k8s labels editor sub-state"
```

---

## Task 8: Pool edit — taints editor sub-state

**Files:** `internal/ui/views/gke_node_pool_edit.go` + test.

```go
const nodePoolEditStateEditingTaints nodePoolEditState = iota + lastExistingState

type GKENodePoolEditView struct {
    // ... existing ...
    taintEditor   *taintedit.Editor
    editingTaints []gcp.NodeTaint // last-saved snapshot
}

func (v *GKENodePoolEditView) openTaintEditor() tea.Cmd {
    initial := v.editingTaints
    if initial == nil {
        initial = append([]gcp.NodeTaint(nil), v.pool.Taints...)
    }
    v.taintEditor = taintedit.New(initial)
    v.taintEditor.SetSize(v.width-4, v.height-8)
    v.state = nodePoolEditStateEditingTaints
    return nil
}
```

`computeEdit` builds `NodePoolEdit.Taints` only when set-equality differs from `v.pool.Taints`. Compare as sorted slices.

Form key handler: when focused on the taints field, Enter or `t` opens the editor.

Diff rendering: per-entry added / removed lines (no "changed" — taint identity is the triple).

Tests parallel labels:

```go
func TestGKENodePoolEdit_TaintsEditedTransitionsToDiff(t *testing.T) {
    pool := &gcp.NodePool{Name: "default", Taints: []gcp.NodeTaint{}}
    v := NewGKENodePoolEditView("proj", "us-central1", "prod", pool)
    v.editingTaints = []gcp.NodeTaint{
        {Key: "dedicated", Value: "gpu", Effect: "NO_SCHEDULE"},
    }
    cmd := v.handleSubmit()
    assert.Nil(t, cmd)
    assert.Equal(t, nodePoolEditStateDiff, v.state)
    require.NotNil(t, v.pendingFields)
    require.NotNil(t, v.pendingFields.Taints)
    require.Len(t, *v.pendingFields.Taints, 1)
}
```

- [ ] **Commit**

```bash
git commit -m "2026-05-21: GKE phase 2d — pool edit: taints editor sub-state"
```

---

## Task 9: Documentation

**Files:** `CLAUDE.md`, `README.md`, `.claude/rules/key-bindings.md`.

Add to key-bindings:

```markdown
### Cluster Edit — Resource Labels editor (sub-state)

| Key | Action |
|-----|--------|
| `Enter` / `l` (on labels field) | Open editor |
| `a` | Add label |
| `e` / `Enter` | Edit selected label |
| `x` / `Delete` | Delete label |
| `Tab` | Switch key/value input |
| `Ctrl+S` | Save and return to form |
| `Esc` | Cancel and return |

### Pool Edit — K8s Labels editor (sub-state)

Same key bindings as cluster labels editor (k8s validation rules).

### Pool Edit — Taints editor (sub-state)

| Key | Action |
|-----|--------|
| `Enter` / `t` (on taints field) | Open editor |
| `a` | Add taint |
| `e` / `Enter` | Edit selected taint |
| `x` / `Delete` | Delete taint |
| `Tab` | Cycle key → value → effect |
| `Ctrl+S` | Save and return |
| `Esc` | Cancel and return |
```

Update CLAUDE.md GKE bullet from `(Phase 1 + 2a + 2b + 2c)` to `(Phase 1 + 2a + 2b + 2c + 2d)` and add a Phase 2d sub-bullet block:

```markdown
  - Phase 2d edit completion:
    - Cluster resource labels editor (GCP label rules)
    - Pool k8s labels editor (DNS subdomain prefixes, dots, uppercase)
    - Pool taints editor (key/value/effect with dropdown)
    - Recurring maintenance windows (days-of-week + start + duration → RRULE)
```

README: append to the GKE entry: "Phase 2d completes the edit flows with maps + taints + recurring maintenance editors."

- [ ] **Commit**

```bash
git add CLAUDE.md README.md .claude/rules/key-bindings.md
git commit -m "2026-05-21: GKE phase 2d — docs"
```

---

## Final integration smoke (manual)

After Task 9 lands, `make run`:

1. Standard cluster → Overview → `e` → labels field → Enter → labeledit opens → add a label → Ctrl+S → returns to form → Ctrl+S → diff shows the new label → Enter → footer task → DONE → details refresh.
2. Same with k8s labels on a pool: Pool Edit → labels field → Enter → labeledit with k8s rules → try `kubernetes.io/role=worker` → confirm validator accepts it.
3. Pool Edit → taints field → Enter → taintedit → add `dedicated=gpu:NO_SCHEDULE` → Ctrl+S → diff → deploy.
4. Cluster Edit → maintenance kind → recurring → pick Mon-Wed-Fri + 03:00 + 4h → diff → deploy.
5. Cluster with existing recurring window: form pre-fills Days/Start/Duration. Submit unchanged → "No changes to apply".
6. Cluster with exotic recurrence (e.g. `FREQ=MONTHLY`): form shows kind=none + placeholder note "recurring policy present but not editable here; use gcloud". Submitting unchanged is a no-op.

---

## Self-review checklist

- [x] Spec coverage: every editor in Design.md has a corresponding task.
- [x] No placeholders: each task has concrete test code + impl shape.
- [x] Type consistency: `MaintenanceWindow`, `RecurringWindow`, `NodeTaint` spelled identically across tasks.
- [x] Validator pluggability: labeledit `SetValidators(Validators{...})` is the integration point.
- [x] HasTextInputFocused delegated to active sub-editor (cluster + pool views both updated).
- [x] No-op submit rejected at form layer with sentinel `errClusterEditNoChanges` / `errNodePoolEditNoChanges` (existing).
- [x] Baseline-defaulting for unrecognized RRULE — same trick as Phase 2c's unknown-logging.
- [x] Day-list validated as non-empty on submit (`errMaintenanceDaysEmpty`).
- [x] No new app-layer handlers or messages — Phase 2c machinery (request/response/poll/sequence) carries through.
