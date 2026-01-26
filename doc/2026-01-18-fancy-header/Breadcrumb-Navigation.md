# Breadcrumb Navigation Guide

## Overview

The fancy header displays hierarchical breadcrumbs that show your current location and navigation context within the application. Breadcrumbs automatically update based on your navigation path.

## Breadcrumb Structure

Breadcrumbs follow this pattern:

```
☁ gcon - Google Cloud Platform TUI  [Project] ▶ [Category] ▶ [Resource1] ▶ [Resource2] ▶ ...
```

### Components

1. **Project**: The currently selected GCP project (blue background)
2. **Category**: The service/resource type you're viewing (green background)
   - Examples: "Compute Engine", "Cloud Storage", "VPC Network"
3. **Resources**: Hierarchical list of resources showing navigation context (yellow background)
   - Can be single or multiple levels deep

## Navigation Examples

### Example 1: VM Instances List

**Path**: Projects → Select Project → VM Instances

**Breadcrumb**:
```
☁ gcon  [my-project] ▶ [Compute Engine]
```

### Example 2: VM Instance Details

**Path**: VM Instances → Select Instance

**Breadcrumb**:
```
☁ gcon  [my-project] ▶ [Compute Engine] ▶ [my-instance]
```

### Example 3: Disk from Instance (Hierarchical)

**Path**: VM Instances → Select Instance → Click Disk Link

**Breadcrumb**:
```
☁ gcon  [my-project] ▶ [Compute Engine] ▶ [my-instance] ▶ [my-disk]
```

This shows the full context: you're viewing a disk that belongs to `my-instance`.

### Example 4: Snapshot from Disk (Deep Hierarchy)

**Path**: Disks → Select Disk → Click Snapshot Link

**Breadcrumb**:
```
☁ gcon  [my-project] ▶ [Compute Engine] ▶ [my-disk] ▶ [snapshot-123]
```

### Example 5: Cloud Storage Bucket

**Path**: Projects → Select Project → Cloud Storage → Select Bucket

**Breadcrumb**:
```
☁ gcon  [my-project] ▶ [Cloud Storage] ▶ [my-bucket]
```

### Example 6: Cloud Storage Objects (Deep Path)

**Path**: Cloud Storage → Select Bucket → Navigate Folders

**Breadcrumb**:
```
☁ gcon  [my-project] ▶ [Cloud Storage] ▶ [my-bucket] ▶ [folder1/folder2/file.txt]
```

## How Breadcrumbs Track Context

### View Stack

The application maintains a view stack that tracks your navigation history. When you:

1. **Navigate forward** (e.g., select an instance): Current view is pushed to stack
2. **Navigate back** (press `Esc`): Previous view is popped from stack

### Parent Context

When viewing related resources (e.g., disk from instance), the breadcrumb logic checks:

1. **Current view**: What are you looking at? (e.g., ViewDiskDetails)
2. **Previous view** (from stack): Where did you come from? (e.g., ViewInstanceDetails)
3. **Selected resources**: What's selected in memory? (e.g., selectedInstance, selectedDisk)

If the previous view is a parent resource, the breadcrumb shows the full path.

### Example Logic

```
Current View: ViewDiskDetails (viewing a disk)
View Stack: [..., ViewInstanceDetails] (came from instance details)
Selected Instance: "web-server-1"
Selected Disk: "boot-disk"

Result: [my-project] ▶ [Compute Engine] ▶ [web-server-1] ▶ [boot-disk]
                                           ^parent        ^current
```

## Navigation Patterns

### Direct Access (No Parent)

When you access a resource directly (e.g., from sidebar or search):

```
Sidebar → Disks → Select Disk
Breadcrumb: [my-project] ▶ [Compute Engine] ▶ [my-disk]
```

No parent instance shown because you didn't navigate through an instance.

### Linked Access (With Parent)

When you access a resource via a link from a parent:

```
VM Instances → my-instance → Click disk link → my-disk
Breadcrumb: [my-project] ▶ [Compute Engine] ▶ [my-instance] ▶ [my-disk]
```

Parent instance shown because you navigated through it.

### Back Navigation

When you press `Esc` to go back:

```
Before: [my-project] ▶ [Compute Engine] ▶ [my-instance] ▶ [my-disk]
Press Esc
After:  [my-project] ▶ [Compute Engine] ▶ [my-instance]
```

The breadcrumb updates to reflect your new position.

## Supported Hierarchies

### Compute Engine

1. **Instance → Disk**
   ```
   [Compute Engine] ▶ [instance-name] ▶ [disk-name]
   ```

2. **Disk → Snapshot**
   ```
   [Compute Engine] ▶ [disk-name] ▶ [snapshot-name]
   ```

3. **Instance → Disk → Snapshot** (future enhancement)
   ```
   [Compute Engine] ▶ [instance-name] ▶ [disk-name] ▶ [snapshot-name]
   ```

### Cloud Storage

1. **Bucket → Folder → File**
   ```
   [Cloud Storage] ▶ [bucket-name] ▶ [folder1/folder2/file.txt]
   ```

## Implementation Details

### Code Location

Breadcrumb logic is implemented in:
- **Header Component**: `internal/ui/components/header.go`
- **Rendering Logic**: `internal/ui/app_render.go` (function `renderHeader()`)
- **Navigation Tracking**: `internal/ui/app_navigation.go`

### Key Functions

```go
// app_render.go - renderHeader()
func (a *App) renderHeader() string {
    // Set project
    a.header.SetProject(a.selectedProject.ID)

    // Set category based on current view
    a.header.SetCategory(category)

    // Build hierarchical resource list
    resources := []string{}
    switch a.currentView {
    case ViewDiskDetails:
        // Check if we came from instance details
        if a.selectedInstance != nil && lastView == ViewInstanceDetails {
            resources = append(resources, a.selectedInstance.Name)
        }
        resources = append(resources, a.selectedDisk.Name)
    }

    a.header.SetResources(resources)
    return a.header.View()
}
```

### View Stack

The view stack is a simple slice that tracks navigation:

```go
type App struct {
    viewStack []ViewType  // Stack of previous views
    currentView ViewType  // Current view
}

// When navigating forward
a.viewStack = append(a.viewStack, a.currentView)
a.currentView = ViewDiskDetails

// When navigating back (Esc key)
if len(a.viewStack) > 0 {
    a.currentView = a.viewStack[len(a.viewStack)-1]
    a.viewStack = a.viewStack[:len(a.viewStack)-1]
}
```

## Future Enhancements

Potential improvements for breadcrumb navigation:

1. **Clickable Breadcrumbs**: Click any segment to jump to that level
2. **Breadcrumb Menu**: Right-click to see available actions
3. **Deeper Hierarchies**: Support 3+ levels (e.g., Instance → Disk → Snapshot)
4. **Breadcrumb History**: See recent navigation paths
5. **Custom Separators**: User-configurable arrow styles

## Troubleshooting

### Breadcrumb Not Showing Parent

**Symptom**: Viewing a disk but parent instance not shown

**Cause**: You accessed the disk directly (sidebar/search) instead of through the instance

**Solution**: This is expected behavior. Navigate through the instance to see the full path.

### Breadcrumb Too Long

**Symptom**: Breadcrumb text is truncated with "..."

**Cause**: Terminal width is narrow, or resource names are very long

**Solution**:
- Widen your terminal window
- Use shorter resource names
- Navigate back to reduce breadcrumb depth

### Breadcrumb Not Updating

**Symptom**: Breadcrumb shows old information after navigation

**Cause**: This would be a bug in the rendering logic

**Solution**: Report as an issue with reproduction steps

## Related Documentation

- [Header Implementation](./Documentation.md)
- [Navigation System](../../CLAUDE.md#key-bindings)
- [Sidebar Navigation](../../internal/ui/components/sidebar/README.md)
