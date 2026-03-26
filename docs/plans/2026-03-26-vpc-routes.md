# VPC Routes Views

## Summary

Add route listing, details, creation, and deletion to the VPC Networking feature set. Routes appear both as a standalone list view (sidebar + command palette) and as a tab on Network Details, matching the existing Subnets pattern.

## GCP API Layer

### New file: `internal/gcp/routes.go`

**Data structures:**

```go
Route struct {
    Name, Description, Network string  // Network = short name
    DestRange string                    // CIDR
    Priority  int64
    NextHop     string                  // resolved display string
    NextHopType string                  // Gateway/Instance/IP/VPNTunnel/Interconnect/ILB
    RouteType   string                  // Static/Subnet/Peering/System (derived)
    Tags        []string
    CreatedAt   string
}

RouteDetails struct {
    // All Route fields plus:
    ID, SelfLink string
    NextHopInstance, NextHopIP, NextHopVPNTunnel string
    NextHopInterconnectAttachment, NextHopILB string
    NextHopGateway, NextHopNetwork, NextHopPeering string
    Warnings []string
}

RouteConfig struct {
    Name, Description, Network string
    DestRange string
    Priority  int64
    Tags      []string
    NextHopType string  // gateway/instance/ip/vpn-tunnel/interconnect/ilb
    NextHopValue string // corresponding value for the type
}
```

**RouteType derivation** (GCP doesn't expose this directly):
- Has `nextHopPeering` set → **Peering**
- Has `nextHopNetwork` set → **Subnet** (auto-created per subnet CIDR)
- Name starts with `default-route-` + nextHop is default gateway → **System**
- Everything else → **Static**

**API methods on ComputeClient:**
- `ListRoutes(ctx, projectID) ([]Route, error)` — all routes, sorted by network then priority
- `ListRoutesByNetwork(ctx, projectID, networkName) ([]Route, error)` — filtered to one network
- `GetRouteDetails(ctx, projectID, routeName) (*RouteDetails, error)`
- `CreateRoute(ctx, projectID, config RouteConfig) error`
- `DeleteRoute(ctx, projectID, routeName) error`

## Views

### Routes List (`internal/ui/views/routes.go`)

- Embeds `TableClickDelegate`
- Table columns: **Name | Network | Dest Range | Priority | Next Hop | Type | Created**
- Type column color-coded: Static = white, Subnet/Peering/System = muted
- Filter support: `network:my-vpc`, `type:static`, `dest:10.0.0.0/8`
- Sort menu on all columns
- Key bindings:
  - `Enter` → Route Details
  - `c` → Route Create
  - `D` → Delete (only for Static type, type-to-confirm)
  - `.` → Action menu
  - `S` → Sort menu
  - `/` → Filter
  - `r` → Refresh

### Route Details (`internal/ui/views/route_details.go`)

- Single scrollable viewport (no tabs — routes are simple resources)
- Sections:
  - **Basic Information**: Name, ID, Description, Created
  - **Routing**: Dest Range, Priority, Next Hop Type, Next Hop Value, Tags
  - **Network**: Navigable link → Network Details
  - **Warnings** (if any from API)
- Key bindings:
  - `D` → Delete (static only, type-to-confirm)
  - `.` → Action menu
  - `r` → Refresh
  - `Tab` → Switch focus (network link / content)
  - `Enter` → Navigate to network (when link focused)

### Route Create (`internal/ui/views/route_create.go`)

- Embeds `CreateViewBase`
- Form sections:
  - **Basic**: Name (required, GCP resource name validation), Description, Network dropdown (async-loaded), Tags (text, comma-separated)
  - **Routing**: Dest Range (required, CIDR validation), Priority (number, 0-65535, default 1000)
  - **Next Hop**: Type dropdown (Gateway / Instance / IP Address / VPN Tunnel / Interconnect Attachment / Internal Load Balancer), Value field (changes based on type)
- Network pre-filled when navigated from Network Details Routes tab

### Network Details — Routes Tab

- Third tab: Details / Subnets / **Routes**
- Lazy-loaded on first visit (create before `updateViewportContent()` per rule #14)
- Uses `links.Links` for navigable route entries
- Columns in link rows: Name, Dest Range, Priority, Next Hop, Type
- `c` → Route Create with network pre-filled
- `Enter` → Route Details

## Messages (`internal/ui/views/route_messages.go`)

```go
RoutesRequestMsg{}
RouteSelectedMsg{Route Route}
RouteCreateRequestMsg{Network string}         // Network optional, pre-fills if set
RouteCreateResultMsg{Error error, Name string}
RouteCreateCanceledMsg{}
RouteDeleteRequestMsg{Name string}
RouteDeleteResultMsg{Error error, Name string}
```

## App Integration

### ViewTypes (`app.go`)
- `ViewRoutes` — standalone routes list
- `ViewRouteDetails` — route details
- `ViewRouteCreate` — route creation form

### App struct fields
- `routesView *views.RoutesView`
- `routeDetailsView *views.RouteDetailsView`
- `routeCreateView *views.RouteCreateView`

### Checklist (per adding-new-views.md)
1. `app.go` — ViewType constants + App fields
2. `app.go` — `getCurrentViewModel()` cases
3. `app_render.go` — `renderCurrentView()` cases
4. `app.go` — `Update()` message handlers for all route messages
5. `app_navigation.go` — handler functions (handleRoutesRequest, handleRouteSelected, etc.)
6. `app_navigation.go` — `clearAllViews()` — nil all three route views
7. `app.go` — `updateViewSizes()` — SetContext on route views
8. `app_navigation.go` — `updateSidebarActiveView()` — all three map to `sidebar.ViewRoutes`
9. Route Create — `HasTextInputFocused()` provided by `CreateViewBase`
10. Command palette — `nav:routes` entry
11. `Init()` idempotent on all three views
12. All messages have both producers and consumers
13. Sidebar guard for Routes must include ViewRoutes, ViewRouteDetails, ViewRouteCreate
14. Network Details Routes tab: create before `updateViewportContent()`
15. Route Details + Routes List — `IsMenuOpen()` for action menu
16. Route Details emits `NetworkSelectedMsg` — add to client resolution chain in handler

### Sidebar (`sidebar/menu.go`)
```
VPC Network (◇)
├── VPC networks (◆) hotkey 'n'
├── Subnets (▫) hotkey 'u'
├── Firewall (▲) hotkey 'f'
└── Routes (→) hotkey 'o'    ← new
```

### Command Palette (`commandpalette/commands.go`)
- `nav:routes` — "VPC Network: Routes", icon `→`

## Testing

### GCP layer (`internal/gcp/routes_test.go`)
- `TestListRoutes` — mock response, verify sort order
- `TestListRoutesByNetwork` — verify filtering
- `TestRouteTypeDerivation` — table-driven for all type inference cases
- `TestCreateRoute` — each next hop type maps to correct API field
- `TestDeleteRoute` — success/error

### View tests (`internal/ui/views/routes_test.go`)
- Routes list: table renders, filter works, delete disabled on non-static
- Route details: sections render, network link present, delete disabled on non-static
- Route create: form validation (CIDR, priority range, required fields), next hop type switching, network pre-fill

### Network details (`internal/ui/views/network_details_test.go` — extend)
- Routes tab renders route entries
- `c` emits `RouteCreateRequestMsg{Network: current}`
- `Enter` emits `RouteSelectedMsg`

## Implementation Order

1. GCP layer — `routes.go` + `routes_test.go`
2. Messages — `route_messages.go`
3. Routes list view — `routes.go` view + test (can parallelize with 4, 5)
4. Route details view — `route_details.go` + test
5. Route create view — `route_create.go` + test
6. Network details Routes tab — extend `network_details.go`
7. App integration — ViewTypes, handlers, render, navigation, clearAllViews, sidebar guards
8. Sidebar + command palette
9. Key bindings doc update
