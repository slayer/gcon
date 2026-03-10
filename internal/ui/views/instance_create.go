package views

import (
	gocontext "context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/forms"
	uierrors "github.com/slayer/gcon/internal/ui/errors"
)

// Internal messages for async data fetching in the create view
type instanceMachineTypesLoadedMsg struct {
	zone  string
	types []gcp.MachineType
}
type instanceMachineTypesErrorMsg struct{ err error }
type instanceNetworksLoadedMsg struct{ networks []gcp.Network }
type instanceSubnetworksLoadedMsg struct{ subnets []gcp.SubnetworkInfo }

// InstanceCreateView allows creating a new VM instance using a form.
// Follows the CreateViewBase pattern — see disk_create.go for reference.
type InstanceCreateView struct {
	CreateViewBase
	computeClient *gcp.ComputeClient
	projectID     string

	// Cached async data to avoid redundant fetches
	machineTypeCache    map[string][]gcp.MachineType
	loadingMachineTypes bool
	networksLoaded      bool

	// Track current zone to detect changes and trigger machine type fetch
	lastZone string
	// Track current image to auto-fill disk size defaults
	lastImage string
}

// NewInstanceCreateView creates a new instance create view.
func NewInstanceCreateView(projectID string, computeClient *gcp.ComputeClient) *InstanceCreateView {
	v := &InstanceCreateView{
		CreateViewBase:   NewCreateViewBase("Creating instance..."),
		computeClient:    computeClient,
		projectID:        projectID,
		machineTypeCache: make(map[string][]gcp.MachineType),
	}
	v.buildForm()
	return v
}

func (v *InstanceCreateView) buildForm() {
	v.Form = buildInstanceForm(forms.FormModeCreate, false)
}

// Init starts the form and triggers initial network loading.
func (v *InstanceCreateView) Init() tea.Cmd {
	cmds := []tea.Cmd{v.CreateViewBase.Init()}
	// Pre-fetch networks so the dropdown is populated
	if !v.networksLoaded {
		cmds = append(cmds, v.fetchNetworks())
	}
	return tea.Batch(cmds...)
}

// Update handles messages for the create view.
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern requires centralized message handling
func (v *InstanceCreateView) Update(msg tea.Msg) tea.Cmd {
	// Let base handle spinner ticks and cancel-during-saving
	if cmd, handled := v.HandleBaseUpdate(msg, InstanceCreateCanceledMsg{}); handled {
		return cmd
	}

	switch msg := msg.(type) {
	case instanceMachineTypesLoadedMsg:
		v.loadingMachineTypes = false
		v.machineTypeCache[msg.zone] = msg.types
		// Update dropdown only if we're still on the same zone
		if v.lastZone == msg.zone {
			v.updateMachineTypeDropdown(msg.types)
		}
		return nil

	case instanceMachineTypesErrorMsg:
		v.loadingMachineTypes = false
		v.SetError(msg.err)
		return nil

	case instanceNetworksLoadedMsg:
		v.networksLoaded = true
		if field := v.Form.GetField("network"); field != nil {
			field.SetOptionsFromStrings(networkDropdownOptions(msg.networks))
		}
		return nil

	case instanceSubnetworksLoadedMsg:
		if field := v.Form.GetField("subnetwork"); field != nil {
			field.SetOptions(subnetworkDropdownOptions(msg.subnets))
		}
		return nil

	case forms.FieldChangedMsg:
		return v.handleFieldChanged(msg)

	case forms.FormSubmitMsg:
		return v.handleSubmit()

	case forms.FormCancelMsg:
		return func() tea.Msg { return InstanceCreateCanceledMsg{} }
	}

	// After delegating to the form, check if zone/image changed
	cmd := v.UpdateForm(msg)
	v.checkFieldChanges()
	return cmd
}

// handleFieldChanged reacts to explicit field change events (e.g., dropdown selection).
func (v *InstanceCreateView) handleFieldChanged(msg forms.FieldChangedMsg) tea.Cmd {
	switch msg.FieldID {
	case "zone":
		if zone, ok := msg.Value.(string); ok && zone != "" {
			return v.onZoneChanged(zone)
		}
	case "image":
		if img, ok := msg.Value.(string); ok {
			v.onImageChanged(img)
		}
	}
	return nil
}

// checkFieldChanges polls form data for zone/image changes that don't emit FieldChangedMsg.
// Text inputs don't emit FieldChangedMsg, but dropdowns do — so this is a safety net.
func (v *InstanceCreateView) checkFieldChanges() {
	if v.Form == nil {
		return
	}
	data := v.Form.GetData()

	// Check zone change
	if zone, ok := data["zone"].(string); ok && zone != "" && zone != v.lastZone {
		// Zone changed but we missed the FieldChangedMsg — unlikely for dropdown but defensive
		v.lastZone = zone
	}

	// Check image change — update default disk size
	if img, ok := data["image"].(string); ok && img != "" && img != v.lastImage {
		v.onImageChanged(img)
	}
}

// onZoneChanged triggers machine type and subnetwork fetching for the selected zone.
func (v *InstanceCreateView) onZoneChanged(zone string) tea.Cmd {
	v.lastZone = zone
	var cmds []tea.Cmd

	// Load machine types (use cache if available)
	if cached, ok := v.machineTypeCache[zone]; ok {
		v.updateMachineTypeDropdown(cached)
	} else if !v.loadingMachineTypes {
		v.loadingMachineTypes = true
		cmds = append(cmds, v.fetchMachineTypes(zone))
	}

	// Load subnetworks for the zone's region
	region := regionFromZoneName(zone)
	cmds = append(cmds, v.fetchSubnetworks(region))

	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

// onImageChanged updates the default disk size based on the selected image.
func (v *InstanceCreateView) onImageChanged(imageValue string) {
	v.lastImage = imageValue
	defaultSize := defaultDiskSizeForImage(imageValue)
	if field := v.Form.GetField("disk_size_gb"); field != nil {
		field.SetValue(defaultSize)
	}
}

func (v *InstanceCreateView) updateMachineTypeDropdown(types []gcp.MachineType) {
	if field := v.Form.GetField("machine_type"); field != nil {
		field.SetOptions(machineTypeDropdownOptions(types))
		// Auto-select e2-medium if available
		for _, mt := range types {
			if mt.Name == "e2-medium" {
				field.SetValue("e2-medium")
				break
			}
		}
	}
}

func (v *InstanceCreateView) fetchMachineTypes(zone string) tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return instanceMachineTypesErrorMsg{err: uierrors.ErrClientNotInitialized}
		}
		types, err := v.computeClient.ListMachineTypes(gocontext.Background(), v.projectID, zone)
		if err != nil {
			return instanceMachineTypesErrorMsg{err: err}
		}
		return instanceMachineTypesLoadedMsg{zone: zone, types: types}
	}
}

func (v *InstanceCreateView) fetchNetworks() tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return instanceNetworksLoadedMsg{networks: nil}
		}
		networks, err := v.computeClient.ListNetworks(gocontext.Background(), v.projectID)
		if err != nil {
			// Non-fatal — keep the "default" option
			return instanceNetworksLoadedMsg{networks: nil}
		}
		return instanceNetworksLoadedMsg{networks: networks}
	}
}

func (v *InstanceCreateView) fetchSubnetworks(region string) tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return instanceSubnetworksLoadedMsg{subnets: nil}
		}
		subnets, err := v.computeClient.ListSubnetworks(gocontext.Background(), v.projectID, region)
		if err != nil {
			// Non-fatal — auto-select will work for auto-mode networks
			return instanceSubnetworksLoadedMsg{subnets: nil}
		}
		return instanceSubnetworksLoadedMsg{subnets: subnets}
	}
}

// handleSubmit validates form data and emits a CreateInstanceMsg.
func (v *InstanceCreateView) handleSubmit() tea.Cmd {
	if errors := v.Form.Validate(); len(errors) > 0 {
		return nil
	}

	data := v.Form.GetData()

	name := ""
	if n, ok := data["name"].(string); ok {
		name = strings.TrimSpace(n)
	}
	zone := ""
	if z, ok := data["zone"].(string); ok {
		zone = z
	}

	// Machine type: prefer custom override, fall back to dropdown
	machineType := ""
	if custom, ok := data["custom_machine_type"].(string); ok && strings.TrimSpace(custom) != "" {
		machineType = strings.TrimSpace(custom)
	} else if mt, ok := data["machine_type"].(string); ok {
		machineType = mt
	}

	// Parse image project/family from "project/family" format
	imageValue := ""
	if iv, ok := data["image"].(string); ok {
		imageValue = iv
	}
	parts := strings.SplitN(imageValue, "/", 2)
	imageProject := ""
	imageFamily := ""
	if len(parts) == 2 {
		imageProject = parts[0]
		imageFamily = parts[1]
	}

	diskSizeGB := int64(10)
	if size, ok := data["disk_size_gb"].(int64); ok && size > 0 {
		diskSizeGB = size
	}

	diskType := "pd-balanced"
	if dt, ok := data["disk_type"].(string); ok && dt != "" {
		diskType = dt
	}

	network := "default"
	if n, ok := data["network"].(string); ok && n != "" {
		network = n
	}

	subnetwork := ""
	if s, ok := data["subnetwork"].(string); ok {
		subnetwork = s
	}

	externalIP := true
	if eip, ok := data["external_ip"].(string); ok {
		externalIP = eip == "ephemeral"
	}

	config := gcp.InstanceCreateConfig{
		Name:         name,
		Zone:         zone,
		MachineType:  machineType,
		ImageProject: imageProject,
		ImageFamily:  imageFamily,
		DiskSizeGB:   diskSizeGB,
		DiskType:     diskType,
		Network:      network,
		Subnetwork:   subnetwork,
		ExternalIP:   externalIP,
	}

	cmd := v.BeginSaving()
	return tea.Batch(cmd, func() tea.Msg {
		return CreateInstanceMsg{Config: config}
	})
}

// GetProjectID returns the project ID for breadcrumbs.
func (v *InstanceCreateView) GetProjectID() string {
	return v.projectID
}

// GetInstanceName returns the entered name, used for result messages.
func (v *InstanceCreateView) GetInstanceName() string {
	if v.Form == nil {
		return ""
	}
	data := v.Form.GetData()
	if name, ok := data["name"].(string); ok {
		return strings.TrimSpace(name)
	}
	return ""
}
