# Subnets List and Management — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add standalone subnet list, details, create, and delete views under Networking in the sidebar.

**Architecture:** Three new view files (list, details, create) plus message types, GCP client methods, and app integration. Follows existing patterns from firewalls (list/details) and snapshot_create (CreateViewBase). Subnet struct gains a Network field; new SubnetDetails struct for the details view.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, GCP Compute API (`google.golang.org/api/compute/v1`), testify

---

### Task 1: GCP Client — Extend Subnet Struct and Add ListAllSubnets

**Files:**
- Modify: `internal/gcp/networks.go`
- Test: `internal/gcp/networks_test.go`

**Step 1: Write tests for Subnet.Network field and ListAllSubnets**

Add to `internal/gcp/networks_test.go`:

```go
func TestSubnetFromAPI_IncludesNetwork(t *testing.T) {
	s := &compute.Subnetwork{
		Name:               "my-subnet",
		Region:             "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1",
		IpCidrRange:        "10.0.0.0/24",
		GatewayAddress:     "10.0.0.1",
		Purpose:            "PRIVATE",
		PrivateIpGoogleAccess: true,
		Network:            "https://www.googleapis.com/compute/v1/projects/p/global/networks/my-vpc",
		CreationTimestamp:   "2024-01-01T00:00:00Z",
	}

	subnet := subnetFromAPI(s)
	assert.Equal(t, "my-vpc", subnet.Network)
	assert.Equal(t, "my-subnet", subnet.Name)
	assert.Equal(t, "us-central1", subnet.Region)
}

func TestSubnetFromAPI_EmptyNetwork(t *testing.T) {
	s := &compute.Subnetwork{
		Name:    "orphan-subnet",
		Network: "",
	}

	subnet := subnetFromAPI(s)
	assert.Equal(t, "", subnet.Network)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/vlad/dev/my/gcon && go test ./internal/gcp/ -run TestSubnetFromAPI_IncludesNetwork -v`
Expected: FAIL — `subnet.Network` is empty string (field doesn't exist yet)

**Step 3: Extend Subnet struct and subnetFromAPI**

In `internal/gcp/networks.go`, add `Network` field to the `Subnet` struct (after `Name`):

```go
type Subnet struct {
	Name                  string
	Network               string // Network name, extracted from network URL
	Region                string
	IPCidrRange           string
	GatewayAddress        string
	Purpose               string
	PrivateIPGoogleAccess bool
	EnableFlowLogs        bool
	CreatedAt             string
}
```

In `subnetFromAPI`, add the Network extraction:

```go
func subnetFromAPI(s *compute.Subnetwork) Subnet {
	enableFlowLogs := false
	if s.LogConfig != nil {
		enableFlowLogs = s.LogConfig.Enable
	}

	return Subnet{
		Name:                  s.Name,
		Network:               extractNameFromURL(s.Network),
		Region:                extractRegionFromURL(s.Region),
		IPCidrRange:           s.IpCidrRange,
		GatewayAddress:        s.GatewayAddress,
		Purpose:               s.Purpose,
		PrivateIPGoogleAccess: s.PrivateIpGoogleAccess,
		EnableFlowLogs:        enableFlowLogs,
		CreatedAt:             s.CreationTimestamp,
	}
}
```

**Step 4: Add ListAllSubnets method**

In `internal/gcp/networks.go`, add after `ListSubnetsByNetwork`:

```go
// ListAllSubnets retrieves all subnets across all regions and networks.
func (c *ComputeClient) ListAllSubnets(ctx context.Context, projectID string) ([]Subnet, error) {
	var subnets []Subnet

	req := c.service.Subnetworks.AggregatedList(projectID)
	err := req.Pages(ctx, func(page *compute.SubnetworkAggregatedList) error {
		for _, scopedList := range page.Items {
			if scopedList.Subnetworks == nil {
				continue
			}
			for _, s := range scopedList.Subnetworks {
				subnets = append(subnets, subnetFromAPI(s))
			}
		}
		return nil
	})
	if err != nil {
		return nil, WrapListError(err, "subnets", projectID)
	}

	sort.Slice(subnets, func(i, j int) bool {
		if subnets[i].Region != subnets[j].Region {
			return subnets[i].Region < subnets[j].Region
		}
		return subnets[i].Name < subnets[j].Name
	})

	return subnets, nil
}
```

**Step 5: Run tests to verify they pass**

Run: `cd /Users/vlad/dev/my/gcon && go test ./internal/gcp/ -run TestSubnetFromAPI -v`
Expected: PASS

**Step 6: Commit**

```
2026-03-26: add Network field to Subnet and ListAllSubnets method
```

---

### Task 2: GCP Client — SubnetDetails, GetSubnetDetails, CreateSubnet, DeleteSubnet

**Files:**
- Modify: `internal/gcp/networks.go`
- Test: `internal/gcp/networks_test.go`

**Step 1: Write tests for subnetDetailsFromAPI**

Add to `internal/gcp/networks_test.go`:

```go
func TestSubnetDetailsFromAPI(t *testing.T) {
	s := &compute.Subnetwork{
		Name:               "test-subnet",
		Id:                 12345,
		Description:        "Test subnet",
		Region:             "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1",
		Network:            "https://www.googleapis.com/compute/v1/projects/p/global/networks/my-vpc",
		IpCidrRange:        "10.0.0.0/24",
		GatewayAddress:     "10.0.0.1",
		Purpose:            "PRIVATE",
		State:              "READY",
		StackType:          "IPV4_ONLY",
		PrivateIpGoogleAccess: true,
		LogConfig: &compute.SubnetworkLogConfig{
			Enable:              true,
			AggregationInterval: "INTERVAL_5_SEC",
			FlowSampling:        0.5,
			Metadata:            "INCLUDE_ALL_METADATA",
			FilterExpr:          "",
		},
		SecondaryIpRanges: []*compute.SubnetworkSecondaryRange{
			{RangeName: "pods", IpCidrRange: "10.1.0.0/16"},
			{RangeName: "services", IpCidrRange: "10.2.0.0/20"},
		},
		CreationTimestamp: "2024-01-01T00:00:00Z",
		SelfLink:          "https://www.googleapis.com/compute/v1/projects/p/regions/us-central1/subnetworks/test-subnet",
	}

	details := subnetDetailsFromAPI(s)
	assert.Equal(t, "test-subnet", details.Name)
	assert.Equal(t, uint64(12345), details.ID)
	assert.Equal(t, "Test subnet", details.Description)
	assert.Equal(t, "us-central1", details.Region)
	assert.Equal(t, "my-vpc", details.Network)
	assert.Equal(t, "10.0.0.0/24", details.IPCidrRange)
	assert.Equal(t, "10.0.0.1", details.GatewayAddress)
	assert.Equal(t, "PRIVATE", details.Purpose)
	assert.Equal(t, "READY", details.Status)
	assert.Equal(t, "IPV4_ONLY", details.StackType)
	assert.True(t, details.PrivateIPGoogleAccess)
	assert.True(t, details.EnableFlowLogs)
	assert.Equal(t, "INTERVAL_5_SEC", details.FlowLogConfig.AggregationInterval)
	assert.Equal(t, 0.5, details.FlowLogConfig.FlowSampling)
	assert.Equal(t, "INCLUDE_ALL_METADATA", details.FlowLogConfig.Metadata)
	assert.Len(t, details.SecondaryIPRanges, 2)
	assert.Equal(t, "pods", details.SecondaryIPRanges[0].Name)
	assert.Equal(t, "10.1.0.0/16", details.SecondaryIPRanges[0].CidrRange)
}

func TestSubnetDetailsFromAPI_NilLogConfig(t *testing.T) {
	s := &compute.Subnetwork{
		Name:      "no-logs",
		LogConfig: nil,
	}

	details := subnetDetailsFromAPI(s)
	assert.False(t, details.EnableFlowLogs)
	assert.Equal(t, FlowLogConfig{}, details.FlowLogConfig)
}

func TestSubnetDetailsFromAPI_NoSecondaryRanges(t *testing.T) {
	s := &compute.Subnetwork{
		Name:              "simple-subnet",
		SecondaryIpRanges: nil,
	}

	details := subnetDetailsFromAPI(s)
	assert.Empty(t, details.SecondaryIPRanges)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/vlad/dev/my/gcon && go test ./internal/gcp/ -run TestSubnetDetailsFromAPI -v`
Expected: FAIL — `subnetDetailsFromAPI` doesn't exist

**Step 3: Add SubnetDetails struct, FlowLogConfig, SecondaryRange, SubnetCreateConfig**

In `internal/gcp/networks.go`, add after the `Subnet` struct:

```go
// SubnetDetails holds comprehensive subnet information for the details view
type SubnetDetails struct {
	ID                    uint64
	Name                  string
	Description           string
	Status                string // READY, etc.
	Region                string
	Network               string // Network name extracted from URL
	IPCidrRange           string
	GatewayAddress        string
	Purpose               string
	StackType             string // IPV4_ONLY, IPV4_IPV6
	IPv6AccessType        string
	IPv6CidrRange         string
	PrivateIPGoogleAccess bool
	EnableFlowLogs        bool
	FlowLogConfig         FlowLogConfig
	SecondaryIPRanges     []SecondaryRange
	CreatedAt             string
	SelfLink              string
}

// FlowLogConfig holds VPC flow log configuration details
type FlowLogConfig struct {
	AggregationInterval string
	FlowSampling        float64
	Metadata            string
	FilterExpr          string
}

// SecondaryRange represents a secondary IP range in a subnet
type SecondaryRange struct {
	Name      string
	CidrRange string
}

// SubnetCreateConfig holds configuration for creating a new subnet
type SubnetCreateConfig struct {
	Name                string
	Description         string
	Network             string // Network name
	Region              string
	CIDRRange           string
	Purpose             string // PRIVATE, REGIONAL_MANAGED_PROXY, INTERNAL_HTTPS_LOAD_BALANCER
	StackType           string // IPV4_ONLY, IPV4_IPV6
	PrivateGoogleAccess bool
	EnableFlowLogs      bool
}
```

**Step 4: Add subnetDetailsFromAPI conversion**

```go
// subnetDetailsFromAPI converts a Compute Engine Subnetwork to SubnetDetails
func subnetDetailsFromAPI(s *compute.Subnetwork) *SubnetDetails {
	enableFlowLogs := false
	var flowLogCfg FlowLogConfig
	if s.LogConfig != nil {
		enableFlowLogs = s.LogConfig.Enable
		flowLogCfg = FlowLogConfig{
			AggregationInterval: s.LogConfig.AggregationInterval,
			FlowSampling:        s.LogConfig.FlowSampling,
			Metadata:            s.LogConfig.Metadata,
			FilterExpr:          s.LogConfig.FilterExpr,
		}
	}

	var secondaryRanges []SecondaryRange
	for _, r := range s.SecondaryIpRanges {
		secondaryRanges = append(secondaryRanges, SecondaryRange{
			Name:      r.RangeName,
			CidrRange: r.IpCidrRange,
		})
	}

	return &SubnetDetails{
		ID:                    s.Id,
		Name:                  s.Name,
		Description:           s.Description,
		Status:                s.State,
		Region:                extractRegionFromURL(s.Region),
		Network:               extractNameFromURL(s.Network),
		IPCidrRange:           s.IpCidrRange,
		GatewayAddress:        s.GatewayAddress,
		Purpose:               s.Purpose,
		StackType:             s.StackType,
		IPv6AccessType:        s.Ipv6AccessType,
		IPv6CidrRange:         s.Ipv6CidrRange,
		PrivateIPGoogleAccess: s.PrivateIpGoogleAccess,
		EnableFlowLogs:        enableFlowLogs,
		FlowLogConfig:         flowLogCfg,
		SecondaryIPRanges:     secondaryRanges,
		CreatedAt:             s.CreationTimestamp,
		SelfLink:              s.SelfLink,
	}
}
```

**Step 5: Add GetSubnetDetails, CreateSubnet, DeleteSubnet methods**

```go
// GetSubnetDetails fetches detailed information for a single subnet
func (c *ComputeClient) GetSubnetDetails(ctx context.Context, projectID, region, subnetName string) (*SubnetDetails, error) {
	s, err := c.service.Subnetworks.Get(projectID, region, subnetName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "subnet", subnetName)
	}
	return subnetDetailsFromAPI(s), nil
}

// CreateSubnet creates a new subnet in the specified region
func (c *ComputeClient) CreateSubnet(ctx context.Context, projectID string, config SubnetCreateConfig) error {
	subnet := &compute.Subnetwork{
		Name:                  config.Name,
		Description:           config.Description,
		Network:               fmt.Sprintf("projects/%s/global/networks/%s", projectID, config.Network),
		IpCidrRange:           config.CIDRRange,
		Purpose:               config.Purpose,
		StackType:             config.StackType,
		PrivateIpGoogleAccess: config.PrivateGoogleAccess,
	}

	if config.EnableFlowLogs {
		subnet.LogConfig = &compute.SubnetworkLogConfig{
			Enable: true,
		}
	}

	_, err := c.service.Subnetworks.Insert(projectID, config.Region, subnet).Context(ctx).Do()
	if err != nil {
		return WrapCreateError(err, "subnet", config.Name)
	}
	return nil
}

// DeleteSubnet deletes a subnet in the specified region
func (c *ComputeClient) DeleteSubnet(ctx context.Context, projectID, region, subnetName string) error {
	_, err := c.service.Subnetworks.Delete(projectID, region, subnetName).Context(ctx).Do()
	if err != nil {
		return WrapDeleteError(err, "subnet", subnetName)
	}
	return nil
}
```

Note: Add `"fmt"` to the imports if not already present.

**Step 6: Run tests**

Run: `cd /Users/vlad/dev/my/gcon && go test ./internal/gcp/ -run TestSubnetDetails -v`
Expected: PASS

**Step 7: Run full gcp package tests**

Run: `cd /Users/vlad/dev/my/gcon && go test ./internal/gcp/ -v`
Expected: PASS — existing tests still work with the new Network field in Subnet

**Step 8: Commit**

```
2026-03-26: add SubnetDetails, GetSubnetDetails, CreateSubnet, DeleteSubnet
```

---

### Task 3: Message Types

**Files:**
- Create: `internal/ui/views/subnet_messages.go`

**Step 1: Create the message types file**

```go
package views

import "github.com/slayer/gcon/internal/gcp"

// SubnetSelectedMsg is sent when a subnet is selected from the list or network details
type SubnetSelectedMsg struct {
	SubnetName string
	Region     string
}

// SubnetCreateRequestMsg is sent to open the subnet creation form
type SubnetCreateRequestMsg struct{}

// SubnetCreateCanceledMsg is sent when user cancels subnet creation
type SubnetCreateCanceledMsg struct{}

// CreateSubnetMsg carries the form data to create a subnet
type CreateSubnetMsg struct {
	Config gcp.SubnetCreateConfig
}

// DeleteSubnetConfirmedMsg is sent when user confirms subnet deletion
type DeleteSubnetConfirmedMsg struct {
	SubnetName string
	Region     string
}

// SubnetActionResultMsg reports the result of an async subnet operation
type SubnetActionResultMsg struct {
	Action  string // "delete", "create"
	Success bool
	Error   error
}
```

**Step 2: Verify it compiles**

Run: `cd /Users/vlad/dev/my/gcon && go build ./internal/ui/views/`
Expected: Success (no errors)

**Step 3: Commit**

```
2026-03-26: add subnet message types
```

---

### Task 4: Subnets List View

**Files:**
- Create: `internal/ui/views/subnets.go`
- Test: `internal/ui/views/subnets_test.go`

**Step 1: Write tests**

Create `internal/ui/views/subnets_test.go`:

```go
package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestSubnetToRow(t *testing.T) {
	subnet := gcp.Subnet{
		Name:                  "my-subnet",
		Network:               "my-vpc",
		Region:                "us-central1",
		IPCidrRange:           "10.0.0.0/24",
		Purpose:               "PRIVATE",
		PrivateIPGoogleAccess: true,
		EnableFlowLogs:        false,
	}

	row := subnetToRow(subnet)
	assert.Equal(t, "my-subnet", row.ID)
	assert.Equal(t, "my-subnet", row.Data[0])
	assert.Equal(t, "my-vpc", row.Data[1])
	assert.Equal(t, "us-central1", row.Data[2])
	assert.Equal(t, "10.0.0.0/24", row.Data[3])
	assert.Contains(t, row.FilterValue, "my-subnet")
	assert.Contains(t, row.FilterValue, "my-vpc")
	assert.Contains(t, row.FilterValue, "us-central1")
}

func TestFormatSubnetPurpose(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"PRIVATE", "Private"},
		{"REGIONAL_MANAGED_PROXY", "Regional Managed Proxy"},
		{"INTERNAL_HTTPS_LOAD_BALANCER", "Internal HTTPS LB"},
		{"PRIVATE_SERVICE_CONNECT", "Private Service Connect"},
		{"", "—"},
		{"UNKNOWN_VALUE", "UNKNOWN_VALUE"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatSubnetPurpose(tt.input))
		})
	}
}

func TestFormatBoolCheck(t *testing.T) {
	assert.Equal(t, "✓", formatBoolCheck(true))
	assert.Equal(t, "—", formatBoolCheck(false))
}

func TestNewSubnetsView(t *testing.T) {
	v := NewSubnetsView("test-project")
	assert.NotNil(t, v)
	assert.Equal(t, "test-project", v.projectID)
	assert.True(t, v.loading)
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/vlad/dev/my/gcon && go test ./internal/ui/views/ -run TestSubnet -v`
Expected: FAIL — functions don't exist

**Step 3: Create subnets.go**

Create `internal/ui/views/subnets.go`. Follow the exact pattern of `firewalls.go`:

```go
package views

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
)

// Internal async messages
type subnetsClientReadyMsg struct{ client *gcp.ComputeClient }
type subnetsLoadedMsg struct{ subnets []gcp.Subnet }
type subnetsErrorMsg struct{ err error }

// SubnetsView displays all subnets across all networks and regions
type SubnetsView struct {
	TableClickDelegate

	computeClient *gcp.ComputeClient
	projectID     string
	ctx           *context.ProgramContext
	table         table.Model
	spinner       spinner.Model
	loading       bool
	err           error
	subnets       []gcp.Subnet
	keys          subnetKeyMap

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool
	pendingDelete     *gcp.Subnet

	width  int
	height int
}

type subnetKeyMap struct {
	Select     key.Binding
	Refresh    key.Binding
	Create     key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
}

func defaultSubnetKeyMap() subnetKeyMap {
	return subnetKeyMap{
		Select:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
		Refresh:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Create:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "create")),
		Delete:     key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete")),
		ActionMenu: key.NewBinding(key.WithKeys("."), key.WithHelp(".", "actions")),
	}
}

func subnetColumns() []table.Column {
	return []table.Column{
		{Title: "Name", Width: 20, Grow: true, Sortable: true},
		{Title: "Network", Width: 15, Sortable: true},
		{Title: "Region", Width: 15, Sortable: true},
		{Title: "CIDR Range", Width: 18, Sortable: true},
		{Title: "Purpose", Width: 22, Sortable: true},
		{Title: "Google Access", Width: 14, Sortable: true},
		{Title: "Flow Logs", Width: 10, Sortable: true},
	}
}

func NewSubnetsView(projectID string) *SubnetsView {
	columns := subnetColumns()
	t := table.NewWithColumns(columns, "Subnets")
	s := components.NewGCPSpinner()

	v := &SubnetsView{
		projectID: projectID,
		table:     t,
		spinner:   s,
		loading:   true,
		keys:      defaultSubnetKeyMap(),
	}

	v.Table = &v.table
	return v
}

func (v *SubnetsView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
	return tea.Batch(
		v.spinner.Tick,
		v.initComputeClient(),
	)
}

func (v *SubnetsView) initComputeClient() tea.Cmd {
	return func() tea.Msg {
		client, err := gcp.NewComputeClient(nil)
		if err != nil {
			return subnetsErrorMsg{err: fmt.Errorf("initialize compute client: %w", err)}
		}
		return subnetsClientReadyMsg{client: client}
	}
}

func (v *SubnetsView) loadSubnets() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return subnetsErrorMsg{err: fmt.Errorf("client not initialized")}
		}
		subnets, err := v.computeClient.ListAllSubnets(nil, v.projectID)
		if err != nil {
			return subnetsErrorMsg{err: err}
		}
		return subnetsLoadedMsg{subnets: subnets}
	}
}

func subnetToRow(s gcp.Subnet) table.Row {
	return table.Row{
		Data: []string{
			s.Name,
			s.Network,
			s.Region,
			s.IPCidrRange,
			formatSubnetPurpose(s.Purpose),
			formatBoolCheck(s.PrivateIPGoogleAccess),
			formatBoolCheck(s.EnableFlowLogs),
		},
		FilterValue: s.Name + " " + s.Network + " " + s.Region + " " + s.IPCidrRange + " " + s.Purpose,
		ID:          s.Name + "/" + s.Region,
	}
}

func formatSubnetPurpose(purpose string) string {
	switch purpose {
	case "PRIVATE":
		return "Private"
	case "REGIONAL_MANAGED_PROXY":
		return "Regional Managed Proxy"
	case "INTERNAL_HTTPS_LOAD_BALANCER":
		return "Internal HTTPS LB"
	case "PRIVATE_SERVICE_CONNECT":
		return "Private Service Connect"
	case "":
		return "—"
	default:
		return purpose
	}
}

func formatBoolCheck(b bool) string {
	if b {
		return "✓"
	}
	return "—"
}

func (v *SubnetsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case subnetsClientReadyMsg:
		v.computeClient = msg.client
		return v.loadSubnets()

	case subnetsLoadedMsg:
		v.loading = false
		v.subnets = msg.subnets
		rows := make([]table.Row, len(v.subnets))
		for i, s := range v.subnets {
			rows[i] = subnetToRow(s)
		}
		v.table.SetRows(rows)
		return nil

	case subnetsErrorMsg:
		v.loading = false
		v.err = msg.err
		return nil

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case confirm.TypeConfirmMsg:
		if v.showDeleteConfirm && v.pendingDelete != nil {
			v.showDeleteConfirm = false
			subnet := v.pendingDelete
			v.pendingDelete = nil
			return func() tea.Msg {
				return DeleteSubnetConfirmedMsg{
					SubnetName: subnet.Name,
					Region:     subnet.Region,
				}
			}
		}
		return nil

	case confirm.TypeCancelMsg:
		v.showDeleteConfirm = false
		v.pendingDelete = nil
		return nil

	case actionmenu.ActionSelectedMsg:
		v.menuOpen = false
		switch msg.Key {
		case 'r':
			return v.Init()
		case 'c':
			return func() tea.Msg { return SubnetCreateRequestMsg{} }
		case 'D':
			return v.initiateDelete()
		}
		return nil

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case table.RowDoubleClickedMsg:
		return v.selectSubnet()

	case tea.KeyMsg:
		// Route to delete confirmation if shown
		if v.showDeleteConfirm && v.deleteConfirm != nil {
			return v.deleteConfirm.Update(msg)
		}

		// Skip during loading
		if v.loading {
			return nil
		}

		// Route to action menu if open
		if v.menuOpen && v.actionMenu != nil {
			return v.actionMenu.Update(msg)
		}

		// Check sort menu and filter
		if v.table.IsSortMenuOpen() || v.table.IsFilterMode() {
			var cmd tea.Cmd
			v.table, cmd = v.table.Update(msg)
			return cmd
		}

		switch {
		case key.Matches(msg, v.keys.Select):
			return v.selectSubnet()
		case key.Matches(msg, v.keys.Refresh):
			return v.Init()
		case key.Matches(msg, v.keys.Create):
			return func() tea.Msg { return SubnetCreateRequestMsg{} }
		case key.Matches(msg, v.keys.Delete):
			return v.initiateDelete()
		case key.Matches(msg, v.keys.ActionMenu):
			return v.openActionMenu()
		}
	}

	// Delegate to table for navigation
	var cmd tea.Cmd
	v.table, cmd = v.table.Update(msg)
	return cmd
}

func (v *SubnetsView) selectSubnet() tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	subnet, ok := v.findSubnetByID(row.ID)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		return SubnetSelectedMsg{
			SubnetName: subnet.Name,
			Region:     subnet.Region,
		}
	}
}

func (v *SubnetsView) findSubnetByID(id string) (gcp.Subnet, bool) {
	for _, s := range v.subnets {
		if s.Name+"/"+s.Region == id {
			return s, true
		}
	}
	return gcp.Subnet{}, false
}

func (v *SubnetsView) openActionMenu() tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	actions := v.buildActions()
	v.actionMenu = actionmenu.New("Subnet Actions", actions)
	v.menuOpen = true
	return nil
}

func (v *SubnetsView) buildActions() []actionmenu.Action {
	return []actionmenu.Action{
		{Key: 'c', Label: "Create Subnet", Enabled: true},
		{Key: 'r', Label: "Refresh", Enabled: true},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
	}
}

func (v *SubnetsView) initiateDelete() tea.Cmd {
	row := v.table.SelectedRow()
	if row == nil {
		return nil
	}
	subnet, ok := v.findSubnetByID(row.ID)
	if !ok {
		return nil
	}

	v.pendingDelete = &subnet

	detailLines := []string{
		fmt.Sprintf("Network: %s", subnet.Network),
		fmt.Sprintf("Region: %s", subnet.Region),
		fmt.Sprintf("CIDR: %s", subnet.IPCidrRange),
	}
	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete Subnet", subnet.Name, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

func (v *SubnetsView) View() string {
	if v.loading && v.computeClient == nil {
		return renderLoading(v.spinner, "Initializing...")
	}
	if v.loading {
		return renderLoading(v.spinner, "Loading subnets...")
	}
	if v.err != nil {
		return "\n" + components.RenderError(v.err)
	}
	if len(v.subnets) == 0 {
		return "\n  No subnets found.\n  Press 'c' to create a subnet or 'esc' to go back."
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	help := helpStyle.Render("\n  enter: details • .: actions • c: create • D: delete • S: sort • /: filter • r: refresh • esc: back")

	mainContent := v.table.View() + help

	if v.menuOpen && v.actionMenu != nil {
		return v.renderWithOverlay(mainContent, v.actionMenu.View())
	}

	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return v.renderWithOverlay(mainContent, v.deleteConfirm.View())
	}

	return mainContent
}

func (v *SubnetsView) renderWithOverlay(bg, fg string) string {
	return actionmenu.RenderCentered(bg, fg, v.width, v.height)
}

func (v *SubnetsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.table.SetSize(ctx.ContentWidth, ctx.ContentHeight-2)
}

// IsMenuOpen implements MenuOpener for Esc routing
func (v *SubnetsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetComputeClient returns the client for reuse by child views
func (v *SubnetsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}
```

**Step 4: Run tests**

Run: `cd /Users/vlad/dev/my/gcon && go test ./internal/ui/views/ -run TestSubnet -v`
Expected: PASS

**Step 5: Run linter on new file**

Run: `cd /Users/vlad/dev/my/gcon && make lint 2>&1 | head -30`
Expected: No errors for subnets.go (there may be unused import warnings until app integration is complete)

**Step 6: Commit**

```
2026-03-26: add subnets list view
```

---

### Task 5: Subnet Details View

**Files:**
- Create: `internal/ui/views/subnet_details.go`
- Test: `internal/ui/views/subnet_details_test.go`

**Step 1: Write tests**

Create `internal/ui/views/subnet_details_test.go`:

```go
package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestNewSubnetDetailsView(t *testing.T) {
	v := NewSubnetDetailsView("test-project", "us-central1", "my-subnet", nil)
	assert.NotNil(t, v)
	assert.Equal(t, "my-subnet", v.subnetName)
	assert.Equal(t, "us-central1", v.region)
	assert.True(t, v.loading)
}

func TestFormatFlowLogSampling(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.5, "50%"},
		{1.0, "100%"},
		{0.0, "0%"},
		{0.25, "25%"},
	}

	for _, tt := range tests {
		result := formatFlowLogSampling(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestRenderSecondaryRanges(t *testing.T) {
	details := &gcp.SubnetDetails{
		SecondaryIPRanges: []gcp.SecondaryRange{
			{Name: "pods", CidrRange: "10.1.0.0/16"},
			{Name: "services", CidrRange: "10.2.0.0/20"},
		},
	}

	v := NewSubnetDetailsView("p", "r", "s", nil)
	v.details = details
	v.width = 80

	content := v.renderSecondaryRanges()
	assert.Contains(t, content, "pods")
	assert.Contains(t, content, "10.1.0.0/16")
	assert.Contains(t, content, "services")
	assert.Contains(t, content, "10.2.0.0/20")
}

func TestRenderSecondaryRanges_Empty(t *testing.T) {
	details := &gcp.SubnetDetails{
		SecondaryIPRanges: nil,
	}

	v := NewSubnetDetailsView("p", "r", "s", nil)
	v.details = details
	v.width = 80

	content := v.renderSecondaryRanges()
	assert.Contains(t, content, "No secondary IP ranges")
}
```

**Step 2: Create subnet_details.go**

Create `internal/ui/views/subnet_details.go`. Follow the pattern from `firewall_details.go` but simpler (no tabs, single viewport):

```go
package views

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components"
	"github.com/slayer/gcon/internal/ui/components/actionmenu"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/links"
	"github.com/slayer/gcon/internal/ui/context"
)

// Internal async messages
type subnetDetailsLoadedMsg struct{ details *gcp.SubnetDetails }
type subnetDetailsErrorMsg struct{ err error }

const subnetDetailsViewportReservedLines = 3 // help line + padding

// SubnetDetailsView displays detailed information about a single subnet
type SubnetDetailsView struct {
	computeClient *gcp.ComputeClient
	projectID     string
	region        string
	subnetName    string
	ctx           *context.ProgramContext

	// Data
	details *gcp.SubnetDetails

	// UI state
	spinner  spinner.Model
	loading  bool
	err      error
	width    int
	height   int
	ready    bool
	viewport viewport.Model

	// Network link (navigable)
	networkLink *links.Links
	linkFocused bool

	// Action menu
	actionMenu *actionmenu.ActionMenu
	menuOpen   bool

	// Delete confirmation
	deleteConfirm     *confirm.TypeConfirmDialog
	showDeleteConfirm bool

	keys subnetDetailsKeyMap
}

type subnetDetailsKeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Refresh    key.Binding
	Delete     key.Binding
	ActionMenu key.Binding
	Tab        key.Binding
}

func defaultSubnetDetailsKeyMap() subnetDetailsKeyMap {
	return subnetDetailsKeyMap{
		Up:         key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:       key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Refresh:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Delete:     key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete")),
		ActionMenu: key.NewBinding(key.WithKeys("."), key.WithHelp(".", "actions")),
		Tab:        key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch focus")),
	}
}

func NewSubnetDetailsView(projectID, region, subnetName string, computeClient *gcp.ComputeClient) *SubnetDetailsView {
	s := components.NewGCPSpinner()

	return &SubnetDetailsView{
		computeClient: computeClient,
		projectID:     projectID,
		region:        region,
		subnetName:    subnetName,
		spinner:       s,
		loading:       true,
		keys:          defaultSubnetDetailsKeyMap(),
		networkLink:   links.New(),
	}
}

func (v *SubnetDetailsView) Init() tea.Cmd {
	v.loading = true
	v.err = nil
	return tea.Batch(v.spinner.Tick, v.loadDetails())
}

func (v *SubnetDetailsView) loadDetails() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return subnetDetailsErrorMsg{err: fmt.Errorf("client not initialized")}
		}
		details, err := v.computeClient.GetSubnetDetails(nil, v.projectID, v.region, v.subnetName)
		if err != nil {
			return subnetDetailsErrorMsg{err: err}
		}
		return subnetDetailsLoadedMsg{details: details}
	}
}

func (v *SubnetDetailsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case subnetDetailsLoadedMsg:
		v.loading = false
		v.details = msg.details
		v.populateNetworkLink()
		v.updateViewportContent()
		return nil

	case subnetDetailsErrorMsg:
		v.loading = false
		v.err = msg.err
		return nil

	case spinner.TickMsg:
		if v.loading {
			var cmd tea.Cmd
			v.spinner, cmd = v.spinner.Update(msg)
			return cmd
		}
		return nil

	case confirm.TypeConfirmMsg:
		if v.showDeleteConfirm {
			v.showDeleteConfirm = false
			return func() tea.Msg {
				return DeleteSubnetConfirmedMsg{
					SubnetName: v.subnetName,
					Region:     v.region,
				}
			}
		}
		return nil

	case confirm.TypeCancelMsg:
		v.showDeleteConfirm = false
		return nil

	case actionmenu.ActionSelectedMsg:
		v.menuOpen = false
		switch msg.Key {
		case 'r':
			return v.Init()
		case 'D':
			return v.initiateDelete()
		}
		return nil

	case actionmenu.ActionMenuClosedMsg:
		v.menuOpen = false
		return nil

	case links.LinkSelectedMsg:
		if v.details != nil {
			return func() tea.Msg {
				return NetworkSelectedMsg{
					Network: gcp.Network{Name: v.details.Network},
				}
			}
		}
		return nil

	case tea.KeyMsg:
		if v.showDeleteConfirm && v.deleteConfirm != nil {
			return v.deleteConfirm.Update(msg)
		}

		if v.loading {
			return nil
		}

		if v.menuOpen && v.actionMenu != nil {
			return v.actionMenu.Update(msg)
		}

		// Tab to toggle focus between link and viewport
		if key.Matches(msg, v.keys.Tab) {
			if v.networkLink != nil && v.networkLink.HasItems() {
				v.linkFocused = !v.linkFocused
				v.updateViewportContent()
			}
			return nil
		}

		if v.linkFocused && v.networkLink != nil && v.networkLink.HasItems() {
			if links.HandleKey(msg) {
				cmd := v.networkLink.Update(msg)
				v.updateViewportContent()
				return cmd
			}
		}

		switch {
		case key.Matches(msg, v.keys.Refresh):
			return v.Init()
		case key.Matches(msg, v.keys.Delete):
			return v.initiateDelete()
		case key.Matches(msg, v.keys.ActionMenu):
			return v.openActionMenu()
		}

		// Scroll viewport
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(msg)
		return cmd
	}

	return nil
}

func (v *SubnetDetailsView) populateNetworkLink() {
	if v.details == nil || v.details.Network == "" {
		v.networkLink.SetItems(nil)
		return
	}
	v.networkLink.SetItems([]links.Link{
		{ID: v.details.Network, Label: v.details.Network, Type: "network"},
	})
}

func (v *SubnetDetailsView) openActionMenu() tea.Cmd {
	actions := []actionmenu.Action{
		{Key: 'r', Label: "Refresh", Enabled: true},
		{Key: 'D', Label: "Delete", Enabled: true, Dangerous: true},
	}
	v.actionMenu = actionmenu.New("Subnet Actions", actions)
	v.menuOpen = true
	return nil
}

func (v *SubnetDetailsView) initiateDelete() tea.Cmd {
	if v.details == nil {
		return nil
	}
	detailLines := []string{
		fmt.Sprintf("Network: %s", v.details.Network),
		fmt.Sprintf("Region: %s", v.details.Region),
		fmt.Sprintf("CIDR: %s", v.details.IPCidrRange),
	}
	v.deleteConfirm = confirm.NewTypeConfirmDialog("Delete Subnet", v.subnetName, detailLines)
	v.showDeleteConfirm = true
	return v.deleteConfirm.Init()
}

func (v *SubnetDetailsView) View() string {
	if v.loading && v.details == nil {
		return renderLoading(v.spinner, "Loading subnet details...")
	}
	if v.err != nil && v.details == nil {
		return "\n" + components.RenderError(v.err)
	}
	if v.details == nil {
		return renderLoading(v.spinner, "No subnet details available...")
	}
	if !v.ready {
		return renderLoading(v.spinner, "Initializing view...")
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6"))
	scrollStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4"))
	scrollInfo := scrollStyle.Render(fmt.Sprintf("%.0f%%", v.viewport.ScrollPercent()*100))
	help := helpStyle.Render("\n  ↑/↓: scroll • tab: focus network • .: actions • D: delete • r: refresh • esc: back") + " " + scrollInfo

	mainContent := v.viewport.View() + help

	if v.menuOpen && v.actionMenu != nil {
		return actionmenu.RenderCentered(mainContent, v.actionMenu.View(), v.width, v.height)
	}
	if v.showDeleteConfirm && v.deleteConfirm != nil {
		return actionmenu.RenderCentered(mainContent, v.deleteConfirm.View(), v.width, v.height)
	}

	return mainContent
}

func (v *SubnetDetailsView) updateViewportContent() {
	if v.details == nil || !v.ready {
		return
	}
	v.viewport.SetContent(v.renderContent())
}

func (v *SubnetDetailsView) renderContent() string {
	d := v.details
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4285F4"))
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(24)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))

	// Header
	b.WriteString(titleStyle.Render(fmt.Sprintf("Subnet: %s", d.Name)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(v.width-4, 60)))
	b.WriteString("\n\n")

	// Basic Information
	b.WriteString(sectionStyle.Render("Basic Information"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Name", d.Name))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "ID", strconv.FormatUint(d.ID, 10)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Status", d.Status))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Region", d.Region))

	// Network as navigable link
	if v.networkLink != nil && v.networkLink.HasItems() {
		b.WriteString(v.networkLink.RenderRow(0, labelStyle.Render("Network:")+" "+d.Network))
	} else {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Network", d.Network))
	}

	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "CIDR Range", d.IPCidrRange))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Gateway", d.GatewayAddress))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Created", d.CreatedAt))
	b.WriteString("\n")

	// Configuration
	b.WriteString(sectionStyle.Render("Configuration"))
	b.WriteString("\n")
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Purpose", formatSubnetPurpose(d.Purpose)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Stack Type", defaultIfEmpty(d.StackType, "IPV4_ONLY")))
	if d.IPv6AccessType != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "IPv6 Access Type", d.IPv6AccessType))
	}
	if d.IPv6CidrRange != "" {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "IPv6 CIDR Range", d.IPv6CidrRange))
	}
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Private Google Access", formatYesNo(d.PrivateIPGoogleAccess)))
	b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Flow Logs", formatYesNo(d.EnableFlowLogs)))
	b.WriteString("\n")

	// Flow Logs Config (if enabled)
	if d.EnableFlowLogs {
		b.WriteString(sectionStyle.Render("Flow Logs Configuration"))
		b.WriteString("\n")
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Aggregation Interval", defaultIfEmpty(d.FlowLogConfig.AggregationInterval, "—")))
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Flow Sampling", formatFlowLogSampling(d.FlowLogConfig.FlowSampling)))
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Metadata", defaultIfEmpty(d.FlowLogConfig.Metadata, "—")))
		if d.FlowLogConfig.FilterExpr != "" {
			b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, "Filter", d.FlowLogConfig.FilterExpr))
		}
		b.WriteString("\n")
	}

	// Secondary IP Ranges
	b.WriteString(v.renderSecondaryRanges())

	return b.String()
}

func (v *SubnetDetailsView) renderSecondaryRanges() string {
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).MarginTop(1)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B6B6B"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9AA0A6")).Width(24)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	var b strings.Builder
	b.WriteString(sectionStyle.Render("Secondary IP Ranges"))
	b.WriteString("\n")

	if v.details == nil || len(v.details.SecondaryIPRanges) == 0 {
		b.WriteString(mutedStyle.Render("  No secondary IP ranges configured"))
		b.WriteString("\n")
		return b.String()
	}

	for _, r := range v.details.SecondaryIPRanges {
		b.WriteString(renderRow(labelStyle, valueStyle, mutedStyle, r.Name, r.CidrRange))
	}

	return b.String()
}

func formatFlowLogSampling(rate float64) string {
	return fmt.Sprintf("%.0f%%", rate*100)
}

func formatYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func (v *SubnetDetailsView) SetContext(ctx *context.ProgramContext) {
	v.ctx = ctx
	v.width = ctx.ContentWidth
	v.height = ctx.ContentHeight
	v.applySize(ctx.ContentWidth, ctx.ContentHeight)
}

func (v *SubnetDetailsView) applySize(width, height int) {
	vpHeight := height - subnetDetailsViewportReservedLines
	if vpHeight < 1 {
		vpHeight = 1
	}
	vpWidth := width - 1
	if vpWidth < 1 {
		vpWidth = 1
	}

	if !v.ready {
		v.viewport = viewport.New(vpWidth, vpHeight)
		v.viewport.Style = lipgloss.NewStyle().Padding(0, 2)
		v.ready = true
	} else {
		v.viewport.Width = vpWidth
		v.viewport.Height = vpHeight
	}

	if v.details != nil {
		v.updateViewportContent()
	}
}

// IsMenuOpen implements MenuOpener for Esc routing
func (v *SubnetDetailsView) IsMenuOpen() bool {
	return v.menuOpen || v.showDeleteConfirm
}

// GetComputeClient returns the client for reuse
func (v *SubnetDetailsView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}
```

**Note:** `formatYesNo` already exists in `network_details.go`. If the compiler complains about a duplicate, remove the one in this file and reuse the existing one. Check with: `grep -rn "func formatYesNo" internal/ui/views/`. If it exists elsewhere, remove the duplicate.

**Step 3: Run tests**

Run: `cd /Users/vlad/dev/my/gcon && go test ./internal/ui/views/ -run "TestNewSubnetDetailsView|TestFormatFlowLog|TestRenderSecondary" -v`
Expected: PASS

**Step 4: Commit**

```
2026-03-26: add subnet details view
```

---

### Task 6: Subnet Create View

**Files:**
- Create: `internal/ui/views/subnet_create.go`
- Test: `internal/ui/views/subnet_create_test.go`

**Step 1: Write tests**

Create `internal/ui/views/subnet_create_test.go`:

```go
package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSubnetCreateView(t *testing.T) {
	v := NewSubnetCreateView("test-project", nil)
	assert.NotNil(t, v)
	assert.NotNil(t, v.Form)
	assert.Equal(t, "test-project", v.projectID)
}

func TestSubnetCreateView_FormFields(t *testing.T) {
	v := NewSubnetCreateView("test-project", nil)

	// Verify required fields exist
	assert.NotNil(t, v.Form.GetField("name"))
	assert.NotNil(t, v.Form.GetField("network"))
	assert.NotNil(t, v.Form.GetField("region"))
	assert.NotNil(t, v.Form.GetField("cidr_range"))
	assert.NotNil(t, v.Form.GetField("purpose"))
	assert.NotNil(t, v.Form.GetField("stack_type"))
	assert.NotNil(t, v.Form.GetField("private_google_access"))
	assert.NotNil(t, v.Form.GetField("flow_logs"))
}

func TestSubnetCreateView_HandleSubmit_ValidationFails(t *testing.T) {
	v := NewSubnetCreateView("test-project", nil)

	// Submit without filling required fields
	cmd := v.handleSubmit()
	assert.Nil(t, cmd) // validation fails, no command returned
}
```

**Step 2: Create subnet_create.go**

```go
package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/forms"
)

// Internal messages
type subnetCreateSuccessMsg struct{}
type subnetCreateErrorMsg struct{ err error }

// networksForSubnetLoadedMsg carries async-loaded network list
type networksForSubnetLoadedMsg struct{ networks []gcp.Network }

// SubnetCreateView provides a form for creating a new subnet
type SubnetCreateView struct {
	CreateViewBase

	computeClient *gcp.ComputeClient
	projectID     string
}

func NewSubnetCreateView(projectID string, computeClient *gcp.ComputeClient) *SubnetCreateView {
	v := &SubnetCreateView{
		CreateViewBase: NewCreateViewBase("Creating subnet..."),
		computeClient:  computeClient,
		projectID:      projectID,
	}

	v.buildForm()
	return v
}

func (v *SubnetCreateView) buildForm() {
	v.Form = forms.NewForm("Create Subnet", forms.FormModeCreate).
		EnableViewport()

	// Basic Settings
	basicSection := forms.NewSection("basic", "Basic Settings").
		AddField(forms.NewTextField("name", "Subnet Name").
			SetRequired(true).
			SetPlaceholder("my-subnet").
			SetHelpText("1-63 characters, lowercase letters, numbers, and hyphens").
			SetValidator(forms.ComposeValidators(
				forms.ValidateRequired,
				forms.ValidateGCPResourceName,
			))).
		AddField(forms.NewTextAreaField("description", "Description").
			SetPlaceholder("Optional description").
			SetRows(2)).
		AddField(forms.NewDropdownField("network", "Network").
			SetRequired(true).
			SetPlaceholder("Loading networks...").
			SetHelpText("VPC network for this subnet")).
		AddField(forms.NewDropdownField("region", "Region").
			SetRequired(true).
			SetOptionsFromStrings(gcp.AllRegions).
			SetHelpText("Region for this subnet"))

	v.Form.AddSection(basicSection)

	// IP Configuration
	ipSection := forms.NewSection("ip", "IP Configuration").
		AddField(forms.NewTextField("cidr_range", "CIDR Range").
			SetRequired(true).
			SetPlaceholder("10.0.0.0/24").
			SetHelpText("Primary IPv4 range in CIDR notation").
			SetValidator(forms.ComposeValidators(
				forms.ValidateRequired,
				forms.ValidateCIDR(),
			))).
		AddField(forms.NewDropdownField("purpose", "Purpose").
			SetOptions([]forms.Option{
				{Value: "PRIVATE", Label: "Private"},
				{Value: "REGIONAL_MANAGED_PROXY", Label: "Regional Managed Proxy"},
				{Value: "INTERNAL_HTTPS_LOAD_BALANCER", Label: "Internal HTTPS Load Balancer"},
			}).
			SetHelpText("Subnet purpose")).
		AddField(forms.NewDropdownField("stack_type", "Stack Type").
			SetOptions([]forms.Option{
				{Value: "IPV4_ONLY", Label: "IPv4 only"},
				{Value: "IPV4_IPV6", Label: "IPv4 and IPv6"},
			}).
			SetHelpText("IP stack type"))

	v.Form.AddSection(ipSection)

	// Access & Logging (collapsible)
	accessSection := forms.NewSection("access", "Access & Logging").
		SetCollapsible(true).
		SetCollapsed(true).
		AddField(forms.NewToggleField("private_google_access", "Private Google Access").
			SetHelpText("Allow VMs to reach Google APIs via internal IPs")).
		AddField(forms.NewToggleField("flow_logs", "VPC Flow Logs").
			SetHelpText("Enable logging of network flows"))

	v.Form.AddSection(accessSection)
}

func (v *SubnetCreateView) Init() tea.Cmd {
	cmds := []tea.Cmd{v.CreateViewBase.Init()}

	// Async-load network list for the dropdown
	if v.computeClient != nil {
		cmds = append(cmds, v.loadNetworks())
	}

	return tea.Batch(cmds...)
}

func (v *SubnetCreateView) loadNetworks() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return networksForSubnetLoadedMsg{networks: nil}
		}
		networks, err := v.computeClient.ListNetworks(nil, v.projectID)
		if err != nil {
			// Non-fatal: dropdown stays with placeholder
			return networksForSubnetLoadedMsg{networks: nil}
		}
		return networksForSubnetLoadedMsg{networks: networks}
	}
}

func (v *SubnetCreateView) Update(msg tea.Msg) tea.Cmd {
	if cmd, handled := v.HandleBaseUpdate(msg, SubnetCreateCanceledMsg{}); handled {
		return cmd
	}

	switch msg := msg.(type) {
	case networksForSubnetLoadedMsg:
		if field := v.Form.GetField("network"); field != nil {
			if msg.networks != nil {
				opts := make([]forms.Option, len(msg.networks))
				for i, n := range msg.networks {
					opts[i] = forms.Option{Value: n.Name, Label: n.Name}
				}
				field.SetOptions(opts)
				field.SetPlaceholder("")
			}
		}
		return nil

	case subnetCreateSuccessMsg:
		return func() tea.Msg {
			return SubnetActionResultMsg{
				Action:  "create",
				Success: true,
			}
		}

	case subnetCreateErrorMsg:
		v.SetError(msg.err)
		return nil

	case forms.FormSubmitMsg:
		return v.handleSubmit()

	case forms.FormCancelMsg:
		return func() tea.Msg { return SubnetCreateCanceledMsg{} }
	}

	return v.UpdateForm(msg)
}

func (v *SubnetCreateView) handleSubmit() tea.Cmd {
	if errors := v.Form.Validate(); len(errors) > 0 {
		return nil
	}

	data := v.Form.GetData()

	config := gcp.SubnetCreateConfig{
		Name:    strings.TrimSpace(data["name"].(string)),
		Network: data["network"].(string),
		Region:  data["region"].(string),
		CIDRRange: strings.TrimSpace(data["cidr_range"].(string)),
	}

	if desc, ok := data["description"].(string); ok {
		config.Description = strings.TrimSpace(desc)
	}
	if purpose, ok := data["purpose"].(string); ok && purpose != "" {
		config.Purpose = purpose
	}
	if stackType, ok := data["stack_type"].(string); ok && stackType != "" {
		config.StackType = stackType
	}
	if pga, ok := data["private_google_access"].(bool); ok {
		config.PrivateGoogleAccess = pga
	}
	if fl, ok := data["flow_logs"].(bool); ok {
		config.EnableFlowLogs = fl
	}

	cmd := v.BeginSaving()

	return tea.Batch(
		cmd,
		func() tea.Msg {
			return CreateSubnetMsg{Config: config}
		},
	)
}

// GetComputeClient returns the compute client for reuse
func (v *SubnetCreateView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}
```

**Important:** Check if `gcp.AllRegions` exists. If not, create a static list or use the regions API. Search with: `grep -rn "AllRegions" internal/gcp/`. If it doesn't exist, use a curated list of common regions as `[]string` options inline, or check if there's a similar pattern (like `AllStorageLocations`).

**Step 3: Run tests**

Run: `cd /Users/vlad/dev/my/gcon && go test ./internal/ui/views/ -run TestSubnetCreate -v`
Expected: PASS (or adjust if `AllRegions` needs a different approach)

**Step 4: Commit**

```
2026-03-26: add subnet create view with form
```

---

### Task 7: App Integration — ViewType, Struct Fields, Rendering

**Files:**
- Modify: `internal/ui/app.go`
- Modify: `internal/ui/app_render.go`

**Step 1: Add ViewType constants**

In `internal/ui/app.go`, add after `ViewNetworkDetails` (line ~51):

```go
ViewSubnets
ViewSubnetDetails
ViewSubnetCreate
```

**Step 2: Add App struct fields**

In `internal/ui/app.go`, add after `firewallDetailsView` (line ~115):

```go
subnetsView       *views.SubnetsView
subnetDetailsView *views.SubnetDetailsView
subnetCreateView  *views.SubnetCreateView
```

**Step 3: Add getCurrentViewModel cases**

In `internal/ui/app.go`, add after the `ViewFirewallDetails` case (line ~375):

```go
case ViewSubnets:
	return a.subnetsView
case ViewSubnetDetails:
	return a.subnetDetailsView
case ViewSubnetCreate:
	return a.subnetCreateView
```

**Step 4: Add updateViewSizes SetContext calls**

In `internal/ui/app.go`, add after the `firewallDetailsView` SetContext block (line ~1245):

```go
if a.subnetsView != nil {
	a.subnetsView.SetContext(a.ctx)
}
if a.subnetDetailsView != nil {
	a.subnetDetailsView.SetContext(a.ctx)
}
if a.subnetCreateView != nil {
	a.subnetCreateView.SetContext(a.ctx)
}
```

**Step 5: Add renderCurrentView cases**

In `internal/ui/app_render.go`, add after the `ViewFirewallDetails` case (line ~251):

```go
case ViewSubnets:
	if a.subnetsView != nil {
		return a.subnetsView.View()
	}
case ViewSubnetDetails:
	if a.subnetDetailsView != nil {
		return a.subnetDetailsView.View()
	}
case ViewSubnetCreate:
	if a.subnetCreateView != nil {
		return a.subnetCreateView.View()
	}
```

**Step 6: Verify it compiles**

Run: `cd /Users/vlad/dev/my/gcon && go build ./...`
Expected: Success

**Step 7: Commit**

```
2026-03-26: add subnet view types, struct fields, and render cases
```

---

### Task 8: App Integration — Navigation Handlers

**Files:**
- Modify: `internal/ui/app.go` (Update message handlers)
- Modify: `internal/ui/app_navigation.go` (handler functions, clearAllViews, sidebar guards, sidebar active view)

**Step 1: Add message handlers in app.go Update()**

In `internal/ui/app.go`, add after the `FirewallActionResultMsg` case (line ~891):

```go
case views.SubnetSelectedMsg:
	return a, a.handleSubnetSelected(msg)

case views.SubnetCreateRequestMsg:
	return a, a.handleSubnetCreateRequest()

case views.SubnetCreateCanceledMsg:
	a.handleSubnetCreateCanceled()
	return a, nil

case views.CreateSubnetMsg:
	return a, a.handleCreateSubnet(msg)

case views.DeleteSubnetConfirmedMsg:
	return a, a.handleDeleteSubnetConfirmed(msg)

case views.SubnetActionResultMsg:
	return a, a.handleSubnetActionResult(msg)
```

**Step 2: Add handler functions in app_navigation.go**

Add to `internal/ui/app_navigation.go`:

```go
func (a *App) handleSubnetSelected(msg views.SubnetSelectedMsg) tea.Cmd {
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewSubnetDetails

	var computeClient *gcp.ComputeClient
	if a.subnetsView != nil {
		computeClient = a.subnetsView.GetComputeClient()
	} else if a.networkDetailsView != nil {
		computeClient = a.networkDetailsView.GetComputeClient()
	}

	a.subnetDetailsView = views.NewSubnetDetailsView(
		a.selectedProject.ID,
		msg.Region,
		msg.SubnetName,
		computeClient,
	)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.subnetDetailsView.Init()
}

func (a *App) handleSubnetCreateRequest() tea.Cmd {
	a.viewStack = append(a.viewStack, a.currentView)
	a.currentView = ViewSubnetCreate

	var computeClient *gcp.ComputeClient
	if a.subnetsView != nil {
		computeClient = a.subnetsView.GetComputeClient()
	}

	a.subnetCreateView = views.NewSubnetCreateView(a.selectedProject.ID, computeClient)
	a.updateSidebarActiveView()
	a.updateViewSizes()
	return a.subnetCreateView.Init()
}

func (a *App) handleSubnetCreateCanceled() {
	a.subnetCreateView = nil
	if len(a.viewStack) > 0 {
		a.currentView = a.viewStack[len(a.viewStack)-1]
		a.viewStack = a.viewStack[:len(a.viewStack)-1]
	}
	a.updateSidebarActiveView()
}

func (a *App) handleCreateSubnet(msg views.CreateSubnetMsg) tea.Cmd {
	var computeClient *gcp.ComputeClient
	if a.subnetCreateView != nil {
		computeClient = a.subnetCreateView.GetComputeClient()
	}
	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	taskID := a.registerRunningTask("Creating subnet...")
	config := msg.Config

	return func() tea.Msg {
		err := computeClient.CreateSubnet(gocontext.Background(), a.selectedProject.ID, config)
		a.finishTask(taskID)
		return views.SubnetActionResultMsg{
			Action:  "create",
			Success: err == nil,
			Error:   err,
		}
	}
}

func (a *App) handleDeleteSubnetConfirmed(msg views.DeleteSubnetConfirmedMsg) tea.Cmd {
	var computeClient *gcp.ComputeClient
	if a.subnetsView != nil {
		computeClient = a.subnetsView.GetComputeClient()
	} else if a.subnetDetailsView != nil {
		computeClient = a.subnetDetailsView.GetComputeClient()
	}

	if computeClient == nil || a.selectedProject == nil {
		return nil
	}

	taskID := a.registerRunningTask("Deleting subnet...")
	projectID := a.selectedProject.ID
	region := msg.Region
	name := msg.SubnetName

	return func() tea.Msg {
		err := computeClient.DeleteSubnet(gocontext.Background(), projectID, region, name)
		a.finishTask(taskID)
		return views.SubnetActionResultMsg{
			Action:  "delete",
			Success: err == nil,
			Error:   err,
		}
	}
}

func (a *App) handleSubnetActionResult(msg views.SubnetActionResultMsg) tea.Cmd {
	if msg.Error != nil {
		a.err = msg.Error

		// Propagate error to active creation view
		if msg.Action == "create" && a.currentView == ViewSubnetCreate && a.subnetCreateView != nil {
			a.subnetCreateView.SetError(msg.Error)
		}
		return nil
	}

	if msg.Action == "delete" {
		// If on details view, pop back to list
		if a.currentView == ViewSubnetDetails {
			if len(a.viewStack) > 0 {
				a.currentView = a.viewStack[len(a.viewStack)-1]
				a.viewStack = a.viewStack[:len(a.viewStack)-1]
			}
			a.subnetDetailsView = nil
			a.updateSidebarActiveView()
		}
		// Refresh list
		if a.subnetsView != nil {
			return a.subnetsView.Init()
		}
	}

	if msg.Action == "create" {
		// Pop back from create view
		if a.currentView == ViewSubnetCreate {
			a.subnetCreateView = nil
			if len(a.viewStack) > 0 {
				a.currentView = a.viewStack[len(a.viewStack)-1]
				a.viewStack = a.viewStack[:len(a.viewStack)-1]
			}
			a.updateSidebarActiveView()
		}
		// Refresh list
		if a.subnetsView != nil {
			return a.subnetsView.Init()
		}
	}

	return nil
}
```

**Step 3: Update clearAllViews**

In `internal/ui/app_navigation.go`, add after `a.firewallDetailsView = nil` (line ~661):

```go
a.subnetsView = nil
a.subnetDetailsView = nil
a.subnetCreateView = nil
```

**Step 4: Update updateSidebarActiveView**

In `internal/ui/app_navigation.go`, add after the firewall case (line ~358):

```go
case ViewSubnets, ViewSubnetDetails, ViewSubnetCreate:
	a.sidebar.SetActiveView(sidebar.ViewSubnets)
```

**Step 5: Add sidebar navigation guard**

In `internal/ui/app_navigation.go`, add a new case in the sidebar navigation switch (after the firewall guard, line ~255):

```go
case sidebar.ViewSubnets:
	if a.currentView != ViewSubnets && a.currentView != ViewSubnetDetails && a.currentView != ViewSubnetCreate {
		a.currentView = ViewSubnets
		a.subnetDetailsView = nil
		a.subnetCreateView = nil
		if a.subnetsView == nil {
			a.subnetsView = views.NewSubnetsView(a.selectedProject.ID)
			a.updateViewSizes()
			cmd = a.subnetsView.Init()
		}
	}
```

**Step 6: Verify it compiles**

Run: `cd /Users/vlad/dev/my/gcon && go build ./...`
Expected: Success (may need `gocontext "context"` import alias in app_navigation.go — check existing pattern)

**Step 7: Commit**

```
2026-03-26: add subnet navigation handlers and app integration
```

---

### Task 9: Sidebar and Command Palette

**Files:**
- Modify: `internal/ui/components/sidebar/menu.go`
- Modify: `internal/ui/components/commandpalette/commands.go`

**Step 1: Add ViewSubnets to sidebar ViewType enum**

In `internal/ui/components/sidebar/menu.go`, add after `ViewFirewall` (line ~23):

```go
ViewSubnets
```

**Step 2: Add icon constant**

In `internal/ui/components/sidebar/menu.go`, add after `IconFirewall` (line ~47):

```go
IconSubnet = "▪" // Subnets leaf
```

Note: if `▪` conflicts with `IconBucket`, use `◆` or another available symbol. Check existing icons to avoid duplicates.

**Step 3: Add Subnets menu item**

In `internal/ui/components/sidebar/menu.go`, in the networking category children (line ~126), add between the VPC networks and Firewall entries:

```go
{ID: "subnets", Label: "Subnets", Icon: IconSubnet, Hotkey: 's', Type: MenuItemLeaf, ViewType: ViewSubnets},
```

**Step 4: Add ViewSubnets to command palette ViewType enum**

In `internal/ui/components/commandpalette/commands.go`, add after `ViewFirewall` (line ~27):

```go
ViewSubnets
```

**Step 5: Add icon constant**

In `internal/ui/components/commandpalette/commands.go`, add an icon for subnets:

```go
IconSubnet = "▪"
```

**Step 6: Add navigation command**

In `internal/ui/components/commandpalette/commands.go`, add after the firewall navigation command (line ~153):

```go
{
	ID:       "nav:subnets",
	Label:    "VPC Network: Subnets",
	Icon:     IconSubnet,
	Type:     CommandTypeNavigation,
	ViewType: ViewSubnets,
	Enabled:  true,
},
```

**Step 7: Update sidebar-to-app ViewType mapping**

Check `internal/ui/app_navigation.go` for where sidebar ViewTypes are mapped. There should be a switch that maps `sidebar.ViewFirewall` → `ViewFirewall`. Add:

```go
case sidebar.ViewSubnets:
```

This was already done in Task 8 Step 5.

Similarly, check `internal/ui/app.go` for where command palette ViewTypes are mapped. Look for the `commandpalette.ViewFirewall` mapping and add:

```go
case commandpalette.ViewSubnets:
	// handled by sidebar navigation
```

**Step 8: Verify it compiles**

Run: `cd /Users/vlad/dev/my/gcon && go build ./...`
Expected: Success

**Step 9: Commit**

```
2026-03-26: add subnets to sidebar and command palette
```

---

### Task 10: Network Details — Navigable Subnet Links

**Files:**
- Modify: `internal/ui/views/network_details.go`

**Step 1: Update the LinkSelectedMsg handler**

In `internal/ui/views/network_details.go`, replace the no-op handler (line ~250-252):

```go
// Before:
case links.LinkSelectedMsg:
	// No-op for now — future subnet details navigation
	return nil

// After:
case links.LinkSelectedMsg:
	if msg.Type == "subnet" {
		if subnet, ok := msg.Data.(gcp.Subnet); ok {
			return func() tea.Msg {
				return SubnetSelectedMsg{
					SubnetName: subnet.Name,
					Region:     subnet.Region,
				}
			}
		}
	}
	return nil
```

**Step 2: Verify the link Data field is set correctly**

Check `populateSubnetLinks()` in `network_details.go`. The existing links should already have `Data: subnet` set. Verify with: `grep -A5 "SetItems" internal/ui/views/network_details.go` around the subnet links population.

If the Data field is not set on the links, update `populateSubnetLinks()` to include it:

```go
linkItems = append(linkItems, links.Link{
	ID:    s.Name,
	Label: s.Name,
	Type:  "subnet",
	Data:  s,  // Ensure this is set
})
```

**Step 3: Verify it compiles**

Run: `cd /Users/vlad/dev/my/gcon && go build ./...`
Expected: Success

**Step 4: Commit**

```
2026-03-26: make subnet links navigable in network details view
```

---

### Task 11: Tests and Lint

**Files:**
- All new and modified files

**Step 1: Run full test suite**

Run: `cd /Users/vlad/dev/my/gcon && make test`
Expected: PASS

**Step 2: Run linter**

Run: `cd /Users/vlad/dev/my/gcon && make lint`
Expected: No errors (fix any that appear)

**Step 3: Fix any issues found**

Common things to check:
- Unused imports in new files
- `formatYesNo` duplicate if it exists in both `network_details.go` and `subnet_details.go`
- Missing `gocontext` import alias in `app_navigation.go`
- Ensure `gcp.AllRegions` exists or replace with alternative

**Step 4: Run tests one more time after fixes**

Run: `cd /Users/vlad/dev/my/gcon && make test && make lint`
Expected: Both PASS

**Step 5: Commit any fixes**

```
2026-03-26: fix lint and test issues for subnets feature
```

---

### Task 12: Update Documentation

**Files:**
- Modify: `CLAUDE.md` — Mark subnets as implemented, add to feature list
- Create: `doc/2026-03-26-subnets/TODO.md`
- Create: `doc/2026-03-26-subnets/Documentation.md`

**Step 1: Update CLAUDE.md**

In the Planned Features section, change:
```
- [ ] Subnets list and management
```
to:
```
- [x] Subnets list and management
```

Add to the Implemented Features section (under VPC Networks):
```
- [x] Subnets list and management
  - List all subnets across networks and regions with table view
  - Subnet details view with secondary IP ranges and flow log config
  - Create subnet with network/region/CIDR/purpose/stack type selection
  - Delete subnet with type-to-confirm
  - Navigate from Network Details subnets tab to subnet details
  - Sidebar entry under Networking, command palette integration
```

**Step 2: Add key bindings to key-bindings rule**

Update `.claude/rules/key-bindings.md` with Subnets View and Subnet Details sections.

**Step 3: Create doc/2026-03-26-subnets/Documentation.md**

Summary of changes, files added/modified, testing instructions.

**Step 4: Commit**

```
2026-03-26: update documentation for subnets feature
```
