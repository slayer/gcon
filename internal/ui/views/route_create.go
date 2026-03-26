package views

import (
	gocontext "context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/forms"
)

// Internal message types for route creation workflow.
type routeCreateSuccessMsg struct{}
type routeCreateErrorMsg struct{ err error }
type networksForRouteLoadedMsg struct {
	networks []gcp.Network
	err      error
}

// RouteCreateView provides a form-based UI for creating new static routes.
type RouteCreateView struct {
	CreateViewBase

	computeClient *gcp.ComputeClient
	projectID     string
	presetNetwork string // pre-fill network when coming from Network Details
}

// NewRouteCreateView creates a new route creation view.
func NewRouteCreateView(projectID string, computeClient *gcp.ComputeClient, presetNetwork string) *RouteCreateView {
	v := &RouteCreateView{
		CreateViewBase: NewCreateViewBase("Creating route..."),
		computeClient:  computeClient,
		projectID:      projectID,
		presetNetwork:  presetNetwork,
	}

	v.buildForm()
	return v
}

// buildForm creates the route creation form with all fields.
func (v *RouteCreateView) buildForm() {
	v.Form = forms.NewForm("Create Route", forms.FormModeCreate).
		EnableViewport()

	// Basic Settings
	basicSection := forms.NewSection("basic", "Basic Settings").
		AddField(forms.NewTextField("name", "Name").
			SetRequired(true).
			SetPlaceholder("my-route").
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
			SetHelpText("VPC network for this route")).
		AddField(forms.NewTextField("tags", "Tags").
			SetPlaceholder("tag1,tag2").
			SetHelpText("Comma-separated network tags; route applies only to instances with these tags"))

	v.Form.AddSection(basicSection)

	// Routing
	routingSection := forms.NewSection("routing", "Routing").
		AddField(forms.NewTextField("dest_range", "Destination Range").
			SetRequired(true).
			SetPlaceholder("10.0.0.0/8").
			SetHelpText("Destination IP range in CIDR notation").
			SetValidator(forms.ComposeValidators(
				forms.ValidateRequired,
				forms.ValidateCIDR(),
			))).
		AddField(forms.NewNumberField("priority", "Priority").
			SetValue(int64(1000)).
			SetHelpText("0-65535, lower values have higher priority").
			SetValidator(forms.ValidateNumber(0, 65535)))

	v.Form.AddSection(routingSection)

	// Next Hop
	nextHopSection := forms.NewSection("nexthop", "Next Hop").
		AddField(forms.NewDropdownField("next_hop_type", "Next Hop Type").
			SetRequired(true).
			SetOptions([]forms.Option{
				{Value: "gateway", Label: "Default Internet Gateway"},
				{Value: "instance", Label: "VM Instance"},
				{Value: "ip", Label: "IP Address"},
				{Value: "vpn-tunnel", Label: "VPN Tunnel"},
				{Value: "interconnect", Label: "Interconnect Attachment"},
				{Value: "ilb", Label: "Internal Load Balancer"},
			})).
		AddField(forms.NewTextField("next_hop_value", "Next Hop Value").
			SetPlaceholder("e.g., 10.0.0.1 or instance name").
			SetHelpText("Instance URL, IP address, VPN tunnel URL, etc. (empty for default gateway)"))

	v.Form.AddSection(nextHopSection)
}

// Init initializes the view and starts loading networks.
func (v *RouteCreateView) Init() tea.Cmd {
	cmds := []tea.Cmd{v.CreateViewBase.Init()}
	if v.computeClient != nil {
		cmds = append(cmds, v.loadNetworks())
	}
	return tea.Batch(cmds...)
}

// loadNetworks fetches available VPC networks for the dropdown.
func (v *RouteCreateView) loadNetworks() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return networksForRouteLoadedMsg{networks: nil}
		}
		networks, err := v.computeClient.ListNetworks(gocontext.Background(), v.projectID)
		if err != nil {
			return networksForRouteLoadedMsg{err: err}
		}
		return networksForRouteLoadedMsg{networks: networks}
	}
}

// Update handles messages for the route creation view.
func (v *RouteCreateView) Update(msg tea.Msg) tea.Cmd {
	// Let base handle spinner ticks and cancel-during-saving
	if cmd, handled := v.HandleBaseUpdate(msg, RouteCreateCanceledMsg{}); handled {
		return cmd
	}

	switch msg := msg.(type) {
	case networksForRouteLoadedMsg:
		if field := v.Form.GetField("network"); field != nil {
			if msg.err != nil {
				field.SetPlaceholder("Failed to load networks")
				v.SetError(msg.err)
			} else if msg.networks != nil {
				opts := make([]forms.Option, len(msg.networks))
				for i, n := range msg.networks {
					opts[i] = forms.Option{Value: n.Name, Label: n.Name}
				}
				field.SetOptions(opts)
				field.SetPlaceholder("")

				// Pre-fill network if specified (e.g., coming from Network Details)
				if v.presetNetwork != "" {
					field.SetValue(v.presetNetwork)
				}
			}
		}
		return nil

	case routeCreateSuccessMsg:
		return func() tea.Msg {
			return RouteCreateResultMsg{Name: v.Form.GetData()["name"].(string)} //nolint:errcheck // name is validated
		}

	case routeCreateErrorMsg:
		v.SetError(msg.err)
		return nil

	case forms.FormSubmitMsg:
		return v.handleSubmit()

	case forms.FormCancelMsg:
		return func() tea.Msg { return RouteCreateCanceledMsg{} }
	}

	return v.UpdateForm(msg)
}

// handleSubmit validates the form and emits a CreateRouteMsg.
func (v *RouteCreateView) handleSubmit() tea.Cmd {
	if errors := v.Form.Validate(); len(errors) > 0 {
		return nil
	}

	data := v.Form.GetData()

	// Safe — form validation above ensures required fields are populated
	name, _ := data["name"].(string)             //nolint:errcheck // validated by form
	network, _ := data["network"].(string)        //nolint:errcheck // validated by form
	destRange, _ := data["dest_range"].(string)   //nolint:errcheck // validated by form
	nextHopType, _ := data["next_hop_type"].(string) //nolint:errcheck // validated by form

	config := gcp.RouteConfig{
		Name:        strings.TrimSpace(name),
		Network:     network,
		DestRange:   strings.TrimSpace(destRange),
		NextHopType: nextHopType,
	}

	if desc, ok := data["description"].(string); ok {
		config.Description = strings.TrimSpace(desc)
	}

	// Parse priority — number field returns int64
	if priority, ok := data["priority"].(int64); ok {
		config.Priority = priority
	} else if priorityStr, ok := data["priority"].(string); ok && priorityStr != "" {
		if p, err := strconv.ParseInt(priorityStr, 10, 64); err == nil {
			config.Priority = p
		}
	}

	// Parse tags — comma-separated string to slice
	if tagsStr, ok := data["tags"].(string); ok && tagsStr != "" {
		rawTags := strings.Split(tagsStr, ",")
		for _, t := range rawTags {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				config.Tags = append(config.Tags, trimmed)
			}
		}
	}

	// Next hop value — for "gateway" type, default to the standard gateway URL
	if nextHopValue, ok := data["next_hop_value"].(string); ok {
		config.NextHopValue = strings.TrimSpace(nextHopValue)
	}
	if nextHopType == "gateway" && config.NextHopValue == "" {
		config.NextHopValue = fmt.Sprintf(
			"projects/%s/global/gateways/default-internet-gateway", v.projectID,
		)
	}

	// Validate that non-gateway types have a value
	if nextHopType != "gateway" && config.NextHopValue == "" {
		v.SetError(fmt.Errorf("next hop value is required for %s type", nextHopType))
		return nil
	}

	cmd := v.BeginSaving()
	return tea.Batch(cmd, func() tea.Msg {
		return CreateRouteMsg{Config: config}
	})
}

// GetComputeClient returns the compute client for reuse.
func (v *RouteCreateView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
}
