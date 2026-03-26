package views

import (
	gocontext "context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/forms"
)

// Internal message types for subnet creation workflow.
type subnetCreateSuccessMsg struct{}
type subnetCreateErrorMsg struct{ err error }
type networksForSubnetLoadedMsg struct {
	networks []gcp.Network
	err      error
}

// SubnetCreateView provides a form-based UI for creating new subnets.
type SubnetCreateView struct {
	CreateViewBase

	computeClient *gcp.ComputeClient
	projectID     string
}

// NewSubnetCreateView creates a new subnet creation view.
func NewSubnetCreateView(projectID string, computeClient *gcp.ComputeClient) *SubnetCreateView {
	v := &SubnetCreateView{
		CreateViewBase: NewCreateViewBase("Creating subnet..."),
		computeClient:  computeClient,
		projectID:      projectID,
	}

	v.buildForm()
	return v
}

// buildForm creates the subnet creation form with all fields.
func (v *SubnetCreateView) buildForm() {
	v.Form = forms.NewForm("Create Subnet", forms.FormModeCreate).
		EnableViewport()

	// Basic Settings
	basicSection := forms.NewSection("basic", "Basic Settings").
		AddField(forms.NewTextField("name", "Name").
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
			SetOptionsFromStrings(gcp.ComputeRegions).
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
			})).
		AddField(forms.NewDropdownField("stack_type", "Stack Type").
			SetOptions([]forms.Option{
				{Value: "IPV4_ONLY", Label: "IPv4 only"},
				{Value: "IPV4_IPV6", Label: "IPv4 and IPv6"},
			}))

	v.Form.AddSection(ipSection)

	// Access & Logging (collapsible, collapsed by default)
	accessSection := forms.NewSection("access", "Access & Logging").
		SetCollapsible(true).
		SetCollapsed(true).
		AddField(forms.NewToggleField("private_google_access", "Private Google Access").
			SetHelpText("Allow VMs to reach Google APIs via internal IPs")).
		AddField(forms.NewToggleField("flow_logs", "VPC Flow Logs").
			SetHelpText("Enable logging of network flows"))

	v.Form.AddSection(accessSection)
}

// Init initializes the view and starts loading networks.
func (v *SubnetCreateView) Init() tea.Cmd {
	cmds := []tea.Cmd{v.CreateViewBase.Init()}
	if v.computeClient != nil {
		cmds = append(cmds, v.loadNetworks())
	}
	return tea.Batch(cmds...)
}

// loadNetworks fetches available VPC networks for the dropdown.
func (v *SubnetCreateView) loadNetworks() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return networksForSubnetLoadedMsg{networks: nil}
		}
		networks, err := v.computeClient.ListNetworks(gocontext.Background(), v.projectID)
		if err != nil {
			return networksForSubnetLoadedMsg{err: err}
		}
		return networksForSubnetLoadedMsg{networks: networks}
	}
}

// Update handles messages for the subnet creation view.
func (v *SubnetCreateView) Update(msg tea.Msg) tea.Cmd {
	// Let base handle spinner ticks and cancel-during-saving
	if cmd, handled := v.HandleBaseUpdate(msg, SubnetCreateCanceledMsg{}); handled {
		return cmd
	}

	switch msg := msg.(type) {
	case networksForSubnetLoadedMsg:
		if field := v.Form.GetField("network"); field != nil {
			if msg.err != nil {
				// Surface the error so user knows networks failed to load
				field.SetPlaceholder("Failed to load networks")
				v.SetError(msg.err)
			} else if msg.networks != nil {
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
			return SubnetActionResultMsg{Action: "create", Success: true}
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

// handleSubmit validates the form and emits a CreateSubnetMsg.
func (v *SubnetCreateView) handleSubmit() tea.Cmd {
	if errors := v.Form.Validate(); len(errors) > 0 {
		return nil
	}

	data := v.Form.GetData()

	// Safe — form validation above ensures required fields are populated
	name, _ := data["name"].(string)            //nolint:errcheck // validated by form
	network, _ := data["network"].(string)      //nolint:errcheck // validated by form
	region, _ := data["region"].(string)        //nolint:errcheck // validated by form
	cidrRange, _ := data["cidr_range"].(string) //nolint:errcheck // validated by form

	config := gcp.SubnetCreateConfig{
		Name:      strings.TrimSpace(name),
		Network:   network,
		Region:    region,
		CIDRRange: strings.TrimSpace(cidrRange),
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
	return tea.Batch(cmd, func() tea.Msg {
		return CreateSubnetMsg{Config: config}
	})
}

// GetComputeClient returns the compute client for reuse.
func (v *SubnetCreateView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}
