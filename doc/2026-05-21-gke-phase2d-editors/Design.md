# GKE Phase 2d — finish the Phase 2c edit punts

## Goal

Land the editors deferred from Phase 2c MVP:

- Cluster **resource labels** (k/v map editor)
- Pool **k8s labels** (k/v map editor — different validation rules than GCP labels)
- Pool **taints** (list of `{key, value, effect}` rows)
- Cluster **recurring maintenance windows** (days-of-week + start/duration → RRULE under the hood)

Plus diff rendering for the new entries (added / removed / changed colour-coded lines).

## Non-goals

- Maintenance exclusion windows (separate "never maintain" intervals) — niche; defer.
- Full RRULE editor — only the most common case (weekly by day-of-week) is exposed via friendly UI. Power users with exotic recurrences (monthly-by-day, every-N-weeks, etc.) must use `gcloud` for now.
- Promoting these editors to a generic `forms.MapField` / `forms.ListField` — the existing `labeledit` component is a stateful sub-view, not a single-row field. Wrapping it inside the forms framework is a separate refactor. For 2d we keep the "sub-state per editor" pattern already established by `instance_editor.go`.

## Reusable component baseline

The codebase already has `internal/ui/components/labeledit/labeledit.go`:

- `New(labels map[string]string) *Editor`
- `GetLabels() map[string]string`
- `Update(tea.Msg) tea.Cmd`, `View() string`, `SetSize(w, h)`
- `IsDirty()`, `IsEditing()`, `HasTextInputFocused()`
- Validation: GCP label key/value patterns (lowercase + numbers + `-_`)

Used today by `instance_editor.go` as a separate state (`stateEditingLabels`). 2d reuses the same integration pattern.

### Validation tweak — k8s labels are NOT GCP labels

K8s label rules (per `validation/IsQualifiedName`):

- Optional DNS subdomain prefix: `[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*` + `/`
- Name part: `[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?`, max 63 chars
- Value: `[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?`, max 63 chars (or empty)

GCP labels: lowercase + numbers + `-_`. **Different.** The cluster's *resource labels* are GCP labels; the pool's `Config.Labels` (k8s node labels) are k8s labels.

→ Extend `labeledit` to accept optional `WithValidators(keyPattern, valuePattern *regexp.Regexp)` so the same component can enforce both rule sets. Default behaviour stays as GCP labels (backwards compatible with instance editor).

## New components

### `taintedit` — node-pool taint editor

Mirrors `labeledit` shape but with three columns per row: key, value, effect. Effect is one of `NO_SCHEDULE / PREFER_NO_SCHEDULE / NO_EXECUTE` — rendered as a small picker (Tab cycles).

Location: `internal/ui/components/taintedit/taintedit.go`.

API surface (mirrors `labeledit`):

```go
type Editor struct { … }
func New(taints []gcp.NodeTaint) *Editor
func (e *Editor) GetTaints() []gcp.NodeTaint
func (e *Editor) Update(tea.Msg) tea.Cmd
func (e *Editor) View() string
func (e *Editor) SetSize(w, h int)
func (e *Editor) IsDirty() bool
func (e *Editor) IsEditing() bool
func (e *Editor) HasTextInputFocused() bool
```

Keys (mirrors labeledit):

| Key | Action |
|---|---|
| `↑/k` / `↓/j` | Move row cursor |
| `a` | Add new taint |
| `e` / `Enter` | Edit selected taint |
| `x` / `Delete` | Delete taint |
| `Tab` | Cycle key → value → effect → key |
| `Ctrl+S` | Save + return to parent edit form |
| `Esc` | Cancel + return |

Taint key/value follow k8s label rules; effect is constrained to the three enums.

## Extending `MaintenanceWindow` for recurring

Today `MaintenanceWindow` has two kinds: `MaintenanceKindNone` and `MaintenanceKindDaily`. Add a third:

```go
const MaintenanceKindRecurring MaintenanceKind = "recurring"

type MaintenanceWindow struct {
    Kind  MaintenanceKind
    Daily string

    // Recurring fields (used when Kind == MaintenanceKindRecurring).
    // We expose a friendly subset; the GCP API needs StartTime/EndTime
    // (RFC3339 datetimes) plus an RRULE. We build them from Days + Start + Duration.
    Days     []string // "MO","TU","WE","TH","FR","SA","SU"
    Start    string   // "HH:MM" UTC
    Duration string   // "Nh" (we accept "1h"-"23h", clamp at submit)
}
```

Builder layer extends `buildSetMaintenancePolicyRequest`:

```go
case MaintenanceKindRecurring:
    // Pick a stable Sunday baseline date (2026-01-04) for the RRULE anchor.
    // Build StartTime / EndTime around the user's Start + Duration.
    // Build Recurrence as "FREQ=WEEKLY;BYDAY=MO,WE,FR"
    policy.Window = &container.MaintenanceWindow{
        RecurringWindow: &container.RecurringTimeWindow{
            Window: &container.TimeWindow{
                StartTime: rfc3339FromBaseline(mw.Start),
                EndTime:   rfc3339FromBaselinePlus(mw.Start, mw.Duration),
            },
            Recurrence: "FREQ=WEEKLY;BYDAY=" + strings.Join(mw.Days, ","),
        },
    }
```

Projection in `gcp.ClusterDetails`:

- Existing: `MaintenanceDaily string` ("" when not daily)
- New: `MaintenanceRecurring *RecurringWindow` (nil unless cluster has a recurring policy that gcon can parse — if RRULE is not `FREQ=WEEKLY;BYDAY=...`, leave nil and treat as "unrecognized" in the form)

Form behaviour for unrecognized recurrence: show a placeholder "Recurring window present but not editable here (use gcloud)" and force kind to "none" if user submits without changing — same baseline trick as cluster-edit's unknown-logging fix.

## UI integration

### Cluster edit (`gke_cluster_edit.go`)

Add two new states:

```go
const (
    clusterEditStateForm clusterEditState = iota
    clusterEditStateEditingLabels    // NEW: labeledit overlay
    clusterEditStateDiff
    clusterEditStateSaving
)
```

Form additions:

- **Basic** section: replace the read-only resource labels display with an interactive read-only entry; pressing `Enter` (or a dedicated `l` key when focused) transitions to `clusterEditStateEditingLabels`. The labeledit component is constructed with the cluster's GCP-label validators (default behaviour).
- **Maintenance** section: extend `maintenance_kind` dropdown with a third option `recurring`. When the user picks `recurring`, the existing `maintenance_daily_start` field is hidden and three new fields appear:
  - `maintenance_days` — multi-select with seven options (Mon–Sun)
  - `maintenance_recurring_start` — text "HH:MM" UTC
  - `maintenance_recurring_duration` — number 1-23 (hours)

`computeEdit` builds a `*gcp.MaintenanceWindow` with `Kind=MaintenanceKindRecurring` and the populated Days/Start/Duration when that kind is selected. If kind didn't change AND days/start/duration didn't change → no diff. Compare day lists as sorted slices.

### Pool edit (`gke_node_pool_edit.go`)

Two new states:

```go
const (
    nodePoolEditStateForm nodePoolEditState = iota
    nodePoolEditStateEditingLabels    // NEW: labeledit (k8s rules)
    nodePoolEditStateEditingTaints    // NEW: taintedit
    nodePoolEditStateDiff
    nodePoolEditStateSaving
)
```

Form additions:

- **Labels** section: previously read-only display; pressing `Enter` (or `l`) opens `labeledit.New(v.pool.Labels)` configured with k8s validators.
- **Taints** section: previously read-only display; pressing `Enter` (or `t`) opens `taintedit.New(v.pool.Taints)`.

`computeEdit` populates `NodePoolEdit.Labels` (non-nil only when k/v map differs from `v.pool.Labels`) and `NodePoolEdit.Taints` (non-nil only when taint list differs — slice element-equal in any order? Yes: compare as sets keyed by `{key, value, effect}`).

## Diff rendering

For maps: lines like
```
  + team=platform
  - env=dev
  ! tier: gold → platinum
```

For taint lists: same shape keyed by `{key, value, effect}`.

For recurring maintenance:
```
  kind:        daily → recurring
  days:        → MO, WE, FR
  start:       → 03:00
  duration:    → 4h
  daily_start: 03:00 →
```

(When transitioning kinds, the obsolete fields render as "→" with empty new value to signal they're being cleared.)

## Key wiring

| Context | Key | Action |
|---|---|---|
| Cluster edit, focused on Resource Labels | `Enter` / `l` | Open labeledit (GCP rules) |
| Cluster edit (labels editor open) | `Ctrl+S` | Save + back to form |
| Cluster edit (labels editor open) | `Esc` | Cancel + back to form |
| Pool edit, focused on K8s Labels | `Enter` / `l` | Open labeledit (k8s rules) |
| Pool edit, focused on Taints | `Enter` / `t` | Open taintedit |
| Both editors | (same labeledit/taintedit bindings) | Add / edit / delete row |

`HasTextInputFocused()` on each edit view must delegate to the active sub-editor when in `*EditingLabels` or `*EditingTaints` state. Otherwise global keys like `q` quit the app while user is typing.

## App-layer wiring

Phase 2c's sequential-step polling (`runGKEEditStep`) already covers multi-endpoint edits, so 2d only changes:

1. `gcp.MaintenanceWindow` extended (one builder change in `gke_edit.go`).
2. `gcp.ClusterDetails.MaintenanceRecurring` projection extended in `gke.go`'s `GetCluster`.
3. `gcp.NodePool.Labels` / `Taints` projections already exist (Phase 2c, Task 3).
4. No new message types — the existing `GKEClusterEditRequestMsg` / `GKENodePoolEditRequestMsg` already carry the right structs (`*gcp.MaintenanceWindow`, `*gcp.NodePoolEdit{Labels, Taints, ...}`).
5. No new handlers — the request handlers route to the existing `SetClusterMaintenancePolicy` / `UpdateNodePoolFields` methods which already accept these fields.

So Phase 2d is mostly **UI + labeledit/taintedit components**, plus the recurring-window projection + builder. The polling / op-tracking layer is untouched.

## Open questions / edge cases

- **K8s label DNS-subdomain prefix** (`my.domain.io/key`): labeledit needs to accept `/` in keys. Current regex only allows `[a-z0-9_-]` — extend the k8s regex to allow uppercase + `.` + `/`.
- **Taint key vs value**: value can be empty (taint with key only). Form must accept empty value.
- **Submit-time validation**: an empty list of selected days when kind=recurring should be rejected ("pick at least one day"). Surface as form error before transitioning to diff.
- **RRULE round-trip**: cluster currently on recurring → user opens form → days/start/duration pre-fill from parsed RRULE → user submits unchanged → no diff. Test this.
- **Taint set-equality**: ordering shouldn't matter, but the GCP API may care. Send the new list verbatim (server treats it as a set anyway).
