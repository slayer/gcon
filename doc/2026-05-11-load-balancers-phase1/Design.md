# Load Balancers — Phase 1 Design

## Scope

**v1 (this spec)**

- **Inventory** — list every forwarding rule in the project, aggregated across
  global + every region. One row per forwarding rule, with a type column
  derived from the target kind + load-balancing scheme.
- **Details** — drill into a single forwarding rule. Resolves the proxy →
  URL map → backend services chain. Three tabs: Overview, Routing, Backends.
- **Delete with dependency cascade** — `D` on the details view computes which
  sub-resources are orphaned by this LB, shows a type-to-confirm dialog
  listing every resource that will be deleted (and every shared one that
  will be kept), then deletes in dependency order. No rollback.

**Deferred to follow-up specs**

- Edit URL map (host / path rules)
- Edit backend-service settings (timeout, session affinity, etc.)
- Attach / detach backends
- Create new LB
- Live backend health (per-instance status from `getHealth`)
- Logical-LB grouping (one row per logical LB instead of per forwarding rule)
- SSL certificate management
- Cloud DNS, Cloud NAT, Cloud CDN (separate "Network Services" leaves)

## UX entry points

A new top-level sidebar category **Network Services** (separate from the
existing VPC Network category) holds future networking leaves. Its first
member is **Load balancing**.

```
◇ VPC Network (V)        ← existing, unchanged
  VPC networks
  Subnets
  Firewall
  Routes

◇ Network Services (N)   ← NEW
  Load balancing (l)     ← Phase 1
  (future) Cloud DNS
  (future) Cloud NAT
  (future) Cloud CDN
```

Command palette: a new **Load balancing** navigation command pointing to
`ViewLoadBalancers`.

## List view (`ViewLoadBalancers`)

### Data source

The Compute API exposes forwarding rules at two scopes:

- `forwardingRules.aggregatedList(project)` — regional forwarding rules,
  grouped by region.
- `globalForwardingRules.list(project)` — global forwarding rules.

The view fires both in parallel and merges the results into a single
flat slice.

### Columns

Cursor-sortable (`S`) and `/`-filterable like other list views.

| Column   | Source                                                                  |
|----------|-------------------------------------------------------------------------|
| Name     | `forwardingRule.name` (default sort)                                    |
| Type     | Derived from `target` URL kind + `loadBalancingScheme` + `IPProtocol`   |
| Scope    | `global` or `<region>` from the aggregated-list key                     |
| IP       | `IPAddress`                                                             |
| Ports    | `portRange` if set, else `ports[]` joined with commas                   |
| Backend  | Last URL segment of the target (proxy name or backend service name)     |

### Type derivation

| Detected target kind + scheme                          | Label                       |
|--------------------------------------------------------|-----------------------------|
| `targetHttpsProxies` + `EXTERNAL_MANAGED`              | `HTTPS (external)`          |
| `targetHttpProxies` + `EXTERNAL_MANAGED`               | `HTTP (external)`           |
| `targetHttpsProxies` + `INTERNAL_MANAGED`              | `HTTPS (internal)`          |
| `targetTcpProxies` + `EXTERNAL_MANAGED`                | `TCP proxy (external)`      |
| `targetSslProxies` + `EXTERNAL_MANAGED`                | `SSL proxy (external)`      |
| `backendServices` directly + `*_MANAGED` scheme        | `Network LB (proxy)`        |
| `backendServices` directly + non-`MANAGED` scheme      | `Network LB (passthrough)`  |
| `targetPools` (legacy)                                 | `Network LB (legacy)`       |
| anything else                                          | raw target-kind string      |

This derivation is a pure function — table-driven tested.

### Lazy backend resolution

To keep the list view fast, the `Backend` column shows only the target's
last URL segment (the immediate "thing the forwarding rule points at").
Full chain resolution happens in the details view. This avoids a
fan-out of API calls per row at list time.

### Standard chrome

- Spinner during load; footer task entry.
- `r` refresh, `/` filter (`field:value` aware), `S` sort menu.
- Inline error with retry on load failure.
- `Esc` returns to the previous view.

## Details view (`ViewLoadBalancerDetails`)

### Load strategy

On entry the view fires parallel `tea.Cmd`s and accumulates per-resource
state. Sections render as soon as their slice of state is ready;
incomplete sections show an inline spinner.

```
forwardingRule (fresh fetch, not list copy)
    ↓ target URL
proxy (HTTP/HTTPS/TCP/SSL) — if target is a proxy
    ↓ urlMap reference (HTTP/HTTPS only)
urlMap
    ↓ defaultService + pathMatchers[].pathRules[].service (deduped)
backend services
    ↓ healthChecks[] (deduped)
health checks

proxy
    ↓ sslCertificates[] (deduped; metadata only)
ssl certificates
```

For Network LBs the forwarding rule's `target` is a `backendService`
directly — the proxy/urlMap legs are skipped.

### Tabs

**Tab 1 — Overview**

- Forwarding rule: name, IP, ports, protocol, scope, network/subnet (if
  internal), labels.
- Type label.
- Proxy info: name, kind, SSL policy (HTTPS only), QUIC enabled (HTTPS
  only).
- SSL certificates: name + type (managed / self-managed). No key
  material.

**Tab 2 — Routing**

- For HTTP(S) LBs: render the URL map as a table.

  ```
  Hosts            Path matcher    Path     → Backend service
  *.example.com    default         /        → my-default-backend
  *.example.com    default         /api/*   → my-api-backend
  internal.x.com   internal        /        → my-internal-backend
  ```

  Plus a `defaultService` row.
- For Network LBs: a single row showing the direct backend service.
- The Backend service column is link-style — `Tab` focuses, `Enter`
  navigates (forward-compatible with a future BackendServiceDetailsView;
  for v1 the Enter is a no-op with a "details view not implemented"
  hint).

**Tab 3 — Backends**

One section per backend service, expanded inline (no further drill-in
in v1):

- Service name, protocol, timeout, session affinity, locality LB policy.
- Backend list: each backend's group URL (instance group or NEG),
  balancing mode, capacity scaler, max RPS / connections.
- Health checks: name, protocol, port, thresholds.

### Standard chrome

- `r` refresh — re-runs the full fetch chain.
- `Tab` cycles focus between tabs, links, content scroll.
- `Esc` returns to the list view.
- `D` triggers delete confirmation.
- `.` opens action menu with a single "Delete" entry.

## Delete with dependency cascade

### Cascade computation (pure)

`ComputeCascade(rule, allFwdRules, allProxies, allURLMaps, allBackends)`
returns a `Cascade` struct:

```go
type Cascade struct {
    Delete []CascadeItem // ordered, root → leaves
    Keep   []CascadeKept // shared items, with the consumers that still need them
}

type CascadeItem struct {
    Kind  string // "forwardingRule", "targetHttpsProxy", "urlMap", "backendService", "healthCheck"
    Name  string
    Scope string // "global" or "<region>"
    URL   string // self-link, for delete sequencing
}

type CascadeKept struct {
    Kind, Name, Scope string
    KeptBecause       []string // human-readable reasons ("referenced by url-map: other-routing")
}
```

Rules:

1. Always cascade the forwarding rule itself.
2. Cascade the target (proxy / backend service / target pool) **iff** no
   other forwarding rule references it.
3. If the target is a proxy with a `urlMap`: cascade the URL map **iff**
   no other proxy references it.
4. Every backend service mentioned in the URL map (or directly by the
   forwarding rule): cascade **iff** no other URL map / forwarding rule
   references it.
5. Every health check on a cascaded backend service: cascade **iff** no
   other backend service references it.
6. **Never** cascade SSL certs, instance groups, NEGs, or VMs — they are
   user-owned resources with independent lifecycles.

The function is purely on inputs — no API calls, table-driven testable.

### Sharing checks

Before showing the confirm dialog the details view issues parallel
sharing-check list calls:

- `forwardingRules.aggregatedList` + `globalForwardingRules.list` (may
  reuse a recent cached list result if implemented)
- `targetHttpsProxies.list` (and the other proxy kinds)
- `urlMaps.list` / `urlMaps.aggregatedList`
- `backendServices.aggregatedList`

A "Computing dependencies…" footer task is shown for this phase. If any
sharing call fails the cascade is aborted with an inline error.

### Confirm dialog

```
╭─ Delete load balancer: my-frontend ──────────────────────╮
│                                                          │
│  Will delete the following resources:                    │
│                                                          │
│    Forwarding rule:     my-frontend          (global)    │
│    Target HTTPS proxy:  my-https-proxy       (global)    │
│    URL map:             my-routing           (global)    │
│    Backend service:     my-api-backend       (global)    │
│    Backend service:     my-static-backend    (global)    │
│    Health check:        api-health           (global)    │
│    Health check:        static-health        (global)    │
│                                                          │
│  Will keep (still in use):                               │
│    Backend service:     shared-backend                   │
│      (referenced by url-map: other-routing)              │
│                                                          │
│  Type the LB name to confirm: [___________________]      │
│                                                          │
│  [ Delete ]   [ Cancel ]                                 │
╰──────────────────────────────────────────────────────────╯
```

Type-to-confirm matches the existing Subnet / Route / Cloud Run delete
pattern.

### Execution order

The Compute API enforces dependency ordering:

1. Delete forwarding rule.
2. Delete target (proxy / direct backend service).
3. Delete URL map (if cascaded).
4. Delete backend services (parallel within this step).
5. Delete health checks (parallel within this step).

Steps 1–3 are sequential; each blocks on its predecessor's success.
Steps 4 and 5 are parallel within the step. A footer task counter
(`Deleting 3/5 resources…`) reports progress.

### Partial-failure semantics

No rollback — the API does not support it. On a mid-cascade failure the
view records every individual delete result and surfaces a summary
inline:

```
Deleted 4 of 5 resources. backend-service "x" failed: <reason>
```

After completion (full or partial) the view navigates back to the list
and triggers a refresh.

## Architecture

```
sidebar Network Services → Load balancing  ──► ViewLoadBalancers
                                                       │ Enter
                                                       ▼
                                          ViewLoadBalancerDetails
                                          ├── parallel fetches via tea.Cmd:
                                          │     forwardingRule → proxy → urlMap
                                          │                          ↓
                                          │                    backendServices
                                          │                          ↓
                                          │                    healthChecks
                                          │     proxy → sslCertificates
                                          │
                                          └── D key → ComputeCascade(pure)
                                                 │
                                                 ▼
                                          Confirm dialog (type-to-confirm)
                                                 │
                                                 ▼
                                          App handles delete sequence:
                                            fwd → proxy → urlMap → backends (parallel)
                                                                  → healthChecks (parallel)
                                                 │
                                                 ▼
                                          back to list + refresh
```

### New files

| File | Purpose |
|---|---|
| `internal/gcp/loadbalancers.go` | `LoadBalancerClient` wrapping the Compute API. Methods for fetch + delete of every resource type used in the cascade. No UI deps. |
| `internal/gcp/loadbalancers_test.go` | Tests for the type-derivation helper and any pure helpers. |
| `internal/ui/views/loadbalancers.go` | `LoadBalancersView` — list with `TableClickDelegate`, sort/filter, Enter → details. |
| `internal/ui/views/loadbalancers_test.go` | Loading state, sort, filter, navigation. |
| `internal/ui/views/loadbalancer_details.go` | `LoadBalancerDetailsView` — tabs + parallel fetch state machine + delete trigger. |
| `internal/ui/views/loadbalancer_details_test.go` | Tab content, fetch state assembly, delete dialog trigger. |
| `internal/ui/views/loadbalancer_messages.go` | `LoadBalancerSelectedMsg`, `LoadBalancerDeletedMsg`, `LoadBalancerDeleteRequestMsg`. |
| `internal/ui/views/loadbalancer_cascade.go` | Pure `ComputeCascade` function. |
| `internal/ui/views/loadbalancer_cascade_test.go` | Table-driven cascade tests. |

### Modified files

- `internal/ui/components/sidebar/menu.go` — new "Network Services" category, "Load balancing" leaf.
- `internal/ui/components/commandpalette/commands.go` — nav command + view enum entry.
- `internal/ui/app.go` — view routing, message handlers, view-instance field, `clearAllViews` entry.
- `internal/ui/app_render.go` — render case.
- `internal/ui/app_navigation.go` — sidebar guards, navigation handlers, sidebar active-view mapping.
- `README.md`, `CLAUDE.md`, `.claude/rules/key-bindings.md` — docs.

## Testing

1. **`ComputeCascade`** — table-driven tests covering:
   - dedicated backend (kept in cascade)
   - shared backend (excluded, `KeptBecause` populated)
   - shared proxy (proxy excluded, so URL map and backends also excluded)
   - legacy `targetPools` (no URL map / no backends to cascade)
   - missing references (URL map referenced but not in input list — graceful skip, no panic)
   - Network LB direct backend (skips proxy/urlMap legs)

2. **Type derivation** — table-driven tests for every type label.

3. **List view** — load success, load error with retry, sort, filter,
   cursor → Enter emits `LoadBalancerSelectedMsg`.

4. **Details view** — partial-fetch state rendering, all-fetched
   rendering, `D` opens confirm dialog with the cascade preview pulled
   from `ComputeCascade`.

5. **App routing** — `LoadBalancerSelectedMsg` navigates to details,
   `LoadBalancerDeletedMsg` returns to list and triggers refresh,
   sidebar guard prevents redundant nav while on details, Esc lands on
   list.

6. **Lint clean** + **full suite green** on Go 1.26.

## Out of scope

Repeated for clarity:

- Edit URL map host/path rules — separate spec.
- Edit backend-service settings — separate spec.
- Attach / detach backends — separate spec.
- Create new LB — separate spec.
- Live backend health (`backendServices.getHealth`) — separate spec.
- Logical-LB grouping (one row per logical LB instead of per forwarding rule) — possible v2 enhancement.
- SSL certificate management.
- Cloud DNS, Cloud NAT, Cloud CDN — separate "Network Services" leaves, separate brainstorms.
