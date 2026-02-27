# Example Designs (gcon-adapted)

Real-world design examples showing how to add new views to gcon using existing patterns and base types.

## Example 1: Adding a GKE Clusters List View

**Requirements**: List GKE clusters with name, location, status, node count, version
**Pattern**: List view with TableClickDelegate (same as instances, disks, buckets, etc.)

**Architecture**:
```go
type ClustersView struct {
    TableClickDelegate              // Mouse click handling
    containerClient *gcp.ContainerClient
    projectID       string
    ctx             *context.ProgramContext
    table           table.Model     // gcon's custom table with sort/filter
    spinner         spinner.Model
    loading         bool
    err             error
    clusters        []gcp.Cluster
    keys            clusterKeyMap
    actionMenu      *actionmenu.ActionMenu
    menuOpen        bool
}

func NewClustersView(projectID string) *ClustersView {
    columns := []table.Column{
        {Title: "Name", Width: 25},
        {Title: "Location", Width: 15},
        {Title: "Status", Width: 12},
        {Title: "Nodes", Width: 8},
        {Title: "Version", Width: 15},
    }
    t := table.New(columns, "GKE Clusters")
    v := &ClustersView{
        projectID: projectID,
        table:     t,
        spinner:   components.NewGCPSpinner(),
        loading:   true,
    }
    v.Table = &v.table  // Wire delegate
    return v
}
```

**Key implementation steps**:
1. Create GCP client method in `internal/gcp/container.go`
2. Create view in `internal/ui/views/clusters.go`
3. Add `ViewClusters` to ViewType enum in `app.go`
4. Add cluster field + navigation handler in `app.go` / `app_navigation.go`
5. Add case in `renderCurrentView()` in `app_render.go`
6. Add to sidebar and command palette
7. See `.claude/rules/adding-new-views.md` for full checklist

## Example 2: Adding a Cloud Run Service Creation View

**Requirements**: Form to create a Cloud Run service (name, region, image, env vars, scaling)
**Pattern**: Creation view with CreateViewBase (same as snapshot_create, disk_create, etc.)

**Architecture**:
```go
type RunServiceCreateView struct {
    CreateViewBase                   // State machine, spinner, error display
    runClient *gcp.RunClient
    projectID string
}

func NewRunServiceCreateView(projectID string,
    client *gcp.RunClient) *RunServiceCreateView {
    v := &RunServiceCreateView{
        CreateViewBase: NewCreateViewBase("Deploying service..."),
        runClient:      client,
        projectID:      projectID,
    }
    v.buildForm()
    return v
}

func (v *RunServiceCreateView) buildForm() {
    v.Form = forms.NewForm("Deploy Service", forms.FormModeCreate).
        EnableViewport()

    basicSection := forms.NewSection("basic", "Service Settings").
        AddField(forms.NewTextField("name", "Service Name").
            SetRequired(true).
            SetValidator(forms.ValidateGCPResourceName)).
        AddField(forms.NewDropdownField("region", "Region").
            SetRequired(true).
            SetOptions(regionOptions())).
        AddField(forms.NewTextField("image", "Container Image").
            SetRequired(true).
            SetPlaceholder("gcr.io/project/image:tag"))

    scalingSection := forms.NewSection("scaling", "Scaling").
        SetCollapsible(true).
        SetCollapsed(true).
        AddField(forms.NewNumberField("min_instances", "Min Instances").
            SetValue(0)).
        AddField(forms.NewNumberField("max_instances", "Max Instances").
            SetValue(100))

    v.Form.AddSection(basicSection).AddSection(scalingSection)
}

func (v *RunServiceCreateView) Update(msg tea.Msg) tea.Cmd {
    if cmd, handled := v.HandleBaseUpdate(msg, RunServiceCreateCanceledMsg{}); handled {
        return cmd
    }
    switch msg.(type) {
    case runServiceCreateSuccessMsg:
        return func() tea.Msg { return RunServiceCreateResultMsg{Success: true} }
    case runServiceCreateErrorMsg:
        v.SetError(msg.err)
        return nil
    case forms.FormSubmitMsg:
        return v.handleSubmit()
    case forms.FormCancelMsg:
        return func() tea.Msg { return RunServiceCreateCanceledMsg{} }
    }
    return v.UpdateForm(msg)
}
```

**What CreateViewBase gives you for free**: `Init()`, `View()`, `SetContext()`, `HasTextInputFocused()`, spinner management, cancel-during-saving, error display.

## Example 3: Adding a Resource Detail View with Tabs

**Requirements**: GKE cluster details with 3 tabs (Details, Node Pools, Workloads)
**Pattern**: Tabbed detail view (same as instance_details, network_details, sql_instance_details)

**Architecture**:
```go
type ClusterDetailsView struct {
    cluster   *gcp.ClusterDetails
    tabs      *tabs.Tabs
    viewport  *viewport.Model
    links     *links.Links          // Clickable links to related resources
    spinner   spinner.Model
    loading   bool
    err       error
    ctx       *context.ProgramContext
    // Tab-specific data
    nodePools []gcp.NodePool
    workloads []gcp.Workload
    keys      clusterDetailsKeyMap
    focusMode detailsFocusMode      // tabs, links, or content
}
```

**Tab rendering pattern**:
```go
func (v *ClusterDetailsView) View() string {
    if v.loading {
        return renderLoading(v.spinner, "Loading cluster details...")
    }
    if v.err != nil {
        return components.RenderError(v.err)
    }

    var b strings.Builder
    b.WriteString(v.tabs.View())
    b.WriteString("\n")

    switch v.tabs.ActiveTab() {
    case 0: // Details
        b.WriteString(v.renderDetailsTab())
    case 1: // Node Pools
        b.WriteString(v.renderNodePoolsTab())
    case 2: // Workloads
        b.WriteString(v.renderWorkloadsTab())
    }

    return b.String()
}
```

## Example 4: Adding an Action with Confirmation

**Requirements**: Delete a GKE cluster with type-to-confirm dialog
**Pattern**: Action with confirm dialog (same as instance delete, firewall delete, etc.)

```go
// In clusters view struct
deleteConfirm     *confirm.TypeConfirmDialog
showDeleteConfirm bool
pendingDelete     *gcp.Cluster

// Key handler
case key.Matches(msg, v.keys.Delete):
    if selected := v.getSelectedCluster(); selected != nil {
        v.pendingDelete = selected
        v.deleteConfirm = confirm.NewTypeConfirmDialog(
            "Delete Cluster",
            fmt.Sprintf("Type '%s' to confirm deletion", selected.Name),
            selected.Name,
        )
        v.showDeleteConfirm = true
        return v.deleteConfirm.Init()
    }

// Confirm result
case confirm.ConfirmedMsg:
    if v.pendingDelete != nil {
        v.showDeleteConfirm = false
        return func() tea.Msg {
            return DeleteClusterConfirmedMsg{
                Name:     v.pendingDelete.Name,
                Location: v.pendingDelete.Location,
            }
        }
    }
```

## Example 5: Adding Sidebar Navigation Entry

**Requirements**: Add "GKE" section to sidebar under a "Containers" category

```go
// In sidebar/menu.go — add ViewType constant
const (
    // ...existing views...
    ViewClusters
)

// In sidebar menu structure — add item
{ID: "containers", Label: "Containers", Icon: IconContainer, Children: []MenuItem{
    {ID: "clusters", Label: "GKE Clusters", ViewType: ViewClusters, Icon: IconCluster},
}}

// In commandpalette/commands.go — add navigation command
{
    ID:       "nav:clusters",
    Label:    "Containers: GKE Clusters",
    Icon:     IconCluster,
    Type:     CommandTypeNavigation,
    ViewType: ViewClusters,
    Enabled:  true,
},
```

## Component Selection Guide (gcon)

| Use Case | gcon Component | Base Type |
|----------|---------------|-----------|
| Resource list | `table.Model` + `TableClickDelegate` | — |
| Resource creation | `forms.Form` + `CreateViewBase` | `CreateViewBase` |
| Resource details | `tabs.Tabs` + `viewport.Model` | — |
| Dangerous action | `confirm.TypeConfirmDialog` | — |
| Context actions | `actionmenu.ActionMenu` | — |
| Text input dialog | `inputdialog.InputDialog` | — |
| Column sorting | `sortmenu.SortMenu` | — |
| Loading state | `components.NewGCPSpinner()` + `renderLoading()` | — |
| Error display | `components.RenderError()` / `RenderInlineError()` | — |
| Navigation | `sidebar.Sidebar` + `commandpalette.CommandPalette` | — |

## Complexity Matrix (gcon views)

| View Type | Key Components | Base Type | Typical Files |
|-----------|---------------|-----------|---------------|
| Resource list | table, spinner, actionMenu | TableClickDelegate | 1 view file + messages |
| Resource details | tabs, viewport, links | — | 1 view file + messages |
| Resource creation | forms, CreateViewBase | CreateViewBase | 1 view file + messages |
| Resource editor | forms (manual) | — | 1 view file + messages |
| Action dialog | confirm, inputdialog | — | Inline in parent view |
