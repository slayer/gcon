# Metadata View Architecture

## Component Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         App (app.go)                             │
│                                                                   │
│  State:                                                          │
│    - currentView: ViewType                                       │
│    - metadataView: *InstanceMetadataView                        │
│    - selectedInstance: *Instance                                 │
│    - selectedProject: *Project                                   │
│                                                                   │
│  Navigation:                                                     │
│    - handleSidebarNavigation(sidebar.ViewMetadata)              │
│    - Back navigation (Esc key)                                   │
│    - Context propagation                                         │
└─────────────────────────────────────────────────────────────────┘
                    │                          │
                    │                          │
        ┌───────────┘                          └──────────┐
        │                                                  │
        ▼                                                  ▼
┌─────────────────────────┐              ┌──────────────────────────────┐
│   Sidebar (sidebar.go)  │              │ InstanceMetadataView         │
│                         │              │ (instance_metadata.go)        │
│  Menu Structure:        │              │                              │
│    Compute Engine       │              │  Dependencies:               │
│      ├─ VM instances    │              │    - ComputeClient           │
│      ├─ Disks           │              │    - MetadataEditor          │
│      └─ Metadata  ◐     │              │    - Viewport                │
│                         │              │                              │
│  ViewTypes:             │              │  State:                      │
│    - ViewInstances      │              │    - instanceMetadata        │
│    - ViewDisks          │              │    - projectMetadata         │
│    - ViewMetadata (NEW) │              │    - fingerprint             │
│                         │              │    - editMode                │
│  Emits:                 │              │                              │
│    NavigateMsg          │              │  Key Features:               │
│                         │              │    - View metadata           │
└─────────────────────────┘              │    - Edit metadata           │
                                         │    - Save to GCP             │
                                         │    - Error handling          │
                                         └──────────────────────────────┘
                                                      │
                                                      │
                                         ┌────────────┴─────────────┐
                                         │                          │
                                         ▼                          ▼
                              ┌──────────────────┐    ┌──────────────────────┐
                              │ MetadataEditor   │    │ ComputeClient        │
                              │ (component)      │    │ (gcp/compute.go)     │
                              │                  │    │                      │
                              │ - Text editing   │    │ API Methods:         │
                              │ - Validation     │    │  - GetInstanceMeta   │
                              │ - Parsing        │    │  - SetInstanceMeta   │
                              │ - Serialization  │    │  - GetProjectMeta    │
                              └──────────────────┘    └──────────────────────┘
```

## Data Flow

### Loading Metadata

```
User Action                     App                    InstanceMetadataView       ComputeClient
     │                          │                              │                        │
     ├─ Select Instance ────────▶                              │                        │
     │                          │                              │                        │
     │                   [Store in selectedInstance]           │                        │
     │                          │                              │                        │
     ├─ Navigate to Metadata ───▶                              │                        │
     │                          │                              │                        │
     │                   [Create metadataView] ────────────────▶                        │
     │                          │                              │                        │
     │                          │                       [Call Init()]                   │
     │                          │                              │                        │
     │                          │                              ├─ GetInstanceMetadata ──▶
     │                          │                              │                        │
     │                          │                              ◀── InstanceMetadata ────┤
     │                          │                              │                        │
     │                          │                              ├─ GetProjectMetadata ───▶
     │                          │                              │                        │
     │                          │                              ◀── ProjectMetadata ─────┤
     │                          │                              │                        │
     │                          │                       [Display metadata]              │
     │                          │                              │                        │
```

### Editing and Saving Metadata

```
User Action                     InstanceMetadataView         MetadataEditor          ComputeClient
     │                                  │                          │                       │
     ├─ Press 'e' ───────────────────▶  │                          │                       │
     │                                  │                          │                       │
     │                          [Enter edit mode]                  │                       │
     │                                  │                          │                       │
     │                                  ├─ SetContent ─────────────▶                       │
     │                                  │                          │                       │
     │                                  │                   [Edit text]                    │
     │                                  │                          │                       │
     ├─ Edit text ───────────────────▶  │                          │                       │
     │                                  │                          │                       │
     │                                  ├─ Forward KeyMsg ─────────▶                       │
     │                                  │                          │                       │
     ├─ Press Ctrl+S ─────────────────▶ │                          │                       │
     │                                  │                          │                       │
     │                          [Trigger save]                     │                       │
     │                                  │                          │                       │
     │                                  ├─ GetContent ─────────────▶                       │
     │                                  │                          │                       │
     │                                  ◀─ Return content ─────────┤                       │
     │                                  │                          │                       │
     │                          [Parse & validate]                 │                       │
     │                                  │                          │                       │
     │                                  ├─ SetInstanceMetadata ─────────────────────────▶  │
     │                                  │    (with fingerprint)    │                       │
     │                                  │                          │                       │
     │                                  ◀─────────────────────────────── Success/Error ───┤
     │                                  │                          │                       │
     │                          [Reload metadata]                  │                       │
     │                                  │                          │                       │
```

## Navigation Flow

```
Projects View
    │
    ├─ Select Project ────────▶ Instances View
    │                               │
    │                               ├─ Select Instance ────▶ Instance Details View
    │                               │                            │
    │                               │                            ├─ Press Esc ─────┐
    │                               │                            │                  │
    │                               │ ◀──────────────────────────┘                  │
    │                               │                                               │
    │                               ├─ Tab (Sidebar focus)                          │
    │                               │     │                                         │
    │                               │     ├─ Navigate to "Metadata"                 │
    │                               │     │                                         │
    │                               │     ▼                                         │
    │                               │  Metadata View ────────────────────────────┐  │
    │                               │     │                                      │  │
    │                               │     ├─ Press 'e' → Edit Mode              │  │
    │                               │     │                                      │  │
    │                               │     ├─ Press Ctrl+S → Save                 │  │
    │                               │     │                                      │  │
    │                               │     ├─ Press Esc ──────────────────────────┘  │
    │                               │     │                                         │
    │                               │ ◀───┘                                         │
    │                               │                                               │
    │ ◀─ Press Esc ─────────────────┘                                               │
    │                                                                               │
```

## State Management

### App State
```go
type App struct {
    // View state
    currentView ViewType              // Current active view
    metadataView *InstanceMetadataView // Metadata view instance

    // Selection state
    selectedProject *Project           // Required for metadata
    selectedInstance *Instance         // Required for metadata

    // Shared context
    ctx *context.ProgramContext       // Propagated to all views
}
```

### Metadata View State
```go
type InstanceMetadataView struct {
    // GCP state
    computeClient *ComputeClient
    projectID string
    zone string
    instanceName string

    // Metadata state
    instanceMetadata *InstanceMetadata
    projectMetadata map[string]string
    fingerprint string               // For optimistic locking

    // UI state
    viewport viewport.Model
    editor MetadataEditor
    loading bool
    saving bool
    editMode bool
    err error
    saveSuccess bool
}
```

## Error Handling

```
Metadata Operation
    │
    ├─ API Call to GCP
    │       │
    │       ├─ Success ────────────▶ metadataLoadedMsg
    │       │                           │
    │       │                           ├─ Update state
    │       │                           └─ Refresh display
    │       │
    │       ├─ Error ──────────────▶ metadataErrorMsg
    │       │                           │
    │       │                           ├─ Display error
    │       │                           └─ Offer retry ('r')
    │       │
    │       └─ 412 Conflict ───────▶ metadataSaveErrorMsg
    │                                   │
    │                                   ├─ Reload metadata
    │                                   └─ Prompt user to retry
    │
```

## Key Integration Points

### 1. Sidebar to App
- Sidebar emits `NavigateMsg{ViewType: sidebar.ViewMetadata}`
- App receives in `Update()` → `handleSidebarNavigation()`

### 2. App to Metadata View
- App creates view: `NewInstanceMetadataView(projectID, zone, name, computeClient)`
- App calls `Init()` to start loading
- App propagates context: `SetContext(ctx)`

### 3. Metadata View to GCP
- View calls `computeClient.GetInstanceMetadata()`
- View calls `computeClient.SetInstanceMetadata()`
- View uses fingerprint for optimistic locking

### 4. Back Navigation
- User presses Esc
- App detects `ViewMetadata` in switch
- App transitions to `ViewInstances`
- App cleans up `metadataView = nil`

## Summary

The metadata view is fully integrated into the application architecture, following the same patterns as other resource views (Disks, Buckets). It properly handles state management, error handling, and resource cleanup while providing a smooth user experience for viewing and editing instance metadata.
