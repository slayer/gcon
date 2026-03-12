package views

import (
	gocontext "context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/diff"
	"github.com/slayer/gcon/internal/ui/components/forms"
	"github.com/slayer/gcon/internal/ui/context"
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
type instanceAddressesLoadedMsg struct{ addresses []gcp.StaticAddress }

// InstanceCreateView allows creating a new VM instance using a form.
// Uses CreateViewBase for form/saving states, adds a confirmation diff step in between.
type InstanceCreateView struct {
	CreateViewBase
	computeClient *gcp.ComputeClient
	projectID     string

	// Confirmation state between form and saving
	showConfirm bool
	diffViewer  *diff.Viewer
	// Holds the parsed config so we don't re-extract on confirm
	pendingConfig *gcp.InstanceCreateConfig

	// Cached async data to avoid redundant fetches
	machineTypeCache     map[string][]gcp.MachineType
	loadingMachineZone   string // zone currently being fetched (empty = idle)
	networksLoaded       bool
	lastLoadedSubnets    []gcp.SubnetworkInfo   // cached subnets for filtering by network
	lastLoadedAddresses  []gcp.StaticAddress     // cached addresses for resolving static IP names

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

// Init starts the form and triggers initial network and machine type loading.
func (v *InstanceCreateView) Init() tea.Cmd {
	cmds := []tea.Cmd{v.CreateViewBase.Init()}
	// Pre-fetch networks so the dropdown is populated
	if !v.networksLoaded {
		cmds = append(cmds, v.fetchNetworks())
	}
	// Pre-fetch machine types for the default zone so the dropdown is populated on first open
	if field := v.Form.GetField("zone"); field != nil {
		if zone, ok := field.GetValue().(string); ok && zone != "" {
			cmds = append(cmds, v.onZoneChanged(zone))
		}
	}
	return tea.Batch(cmds...)
}

// View renders the current state: form, confirmation, or saving.
func (v *InstanceCreateView) View() string {
	// Confirmation takes priority over the base form/saving view
	if v.showConfirm && v.diffViewer != nil {
		return v.diffViewer.View()
	}
	return v.CreateViewBase.View()
}

// SetContext updates dimensions and propagates to form and diff viewer.
func (v *InstanceCreateView) SetContext(ctx *context.ProgramContext) {
	v.CreateViewBase.SetContext(ctx)
	if v.diffViewer != nil {
		v.diffViewer.SetSize(ctx.ContentWidth-8, ctx.ContentHeight-10)
	}
}

// Update handles messages for the create view.
//
//nolint:gocognit,cyclop // Bubble Tea Update pattern requires centralized message handling
func (v *InstanceCreateView) Update(msg tea.Msg) tea.Cmd {
	// Let base handle spinner ticks and cancel-during-saving
	if cmd, handled := v.HandleBaseUpdate(msg, InstanceCreateCanceledMsg{}); handled {
		return cmd
	}

	// Handle confirmation state messages
	if v.showConfirm {
		switch msg.(type) {
		case diff.ConfirmMsg:
			return v.confirmCreate()
		case diff.CancelMsg:
			v.showConfirm = false
			v.diffViewer = nil
			v.pendingConfig = nil
			return nil
		}
		// Delegate key events to diff viewer
		if v.diffViewer != nil {
			return v.diffViewer.Update(msg)
		}
		return nil
	}

	switch msg := msg.(type) {
	case instanceMachineTypesLoadedMsg:
		v.machineTypeCache[msg.zone] = msg.types
		if v.loadingMachineZone == msg.zone {
			v.loadingMachineZone = ""
		}
		// Update dropdown only if we're still on the same zone
		if v.lastZone == msg.zone {
			v.updateMachineTypeDropdown(msg.types)
		}
		return nil

	case instanceMachineTypesErrorMsg:
		v.loadingMachineZone = ""
		v.SetError(msg.err)
		return nil

	case instanceNetworksLoadedMsg:
		v.networksLoaded = true
		// Only update options when fetch succeeded — nil means error, keep "default"
		if msg.networks != nil {
			if field := v.Form.GetField("network"); field != nil {
				field.SetOptionsFromStrings(networkDropdownOptions(msg.networks))
			}
		}
		return nil

	case instanceSubnetworksLoadedMsg:
		v.lastLoadedSubnets = msg.subnets
		v.filterSubnetsByNetwork()
		return nil

	case instanceAddressesLoadedMsg:
		v.lastLoadedAddresses = msg.addresses
		if field := v.Form.GetField("external_ip"); field != nil {
			// Preserve current selection across option refresh
			current := field.GetValue()
			field.SetOptions(externalIPDropdownOptions(msg.addresses))
			if s, ok := current.(string); ok && s != "" {
				field.SetValue(s)
			}
		}
		return nil

	case forms.FieldChangedMsg:
		return v.handleFieldChanged(msg)

	case forms.FormSubmitMsg:
		return v.handleSubmit()

	case forms.FormCancelMsg:
		return func() tea.Msg { return InstanceCreateCanceledMsg{} }
	}

	cmd := v.UpdateForm(msg)
	v.checkImageChange()
	return cmd
}

// handleFieldChanged reacts to explicit field change events (e.g., dropdown selection).
func (v *InstanceCreateView) handleFieldChanged(msg forms.FieldChangedMsg) tea.Cmd {
	switch msg.FieldID {
	case "zone":
		if zone, ok := msg.Value.(string); ok && zone != "" {
			return v.onZoneChanged(zone)
		}
	case "network":
		v.filterSubnetsByNetwork()
	case "image":
		if img, ok := msg.Value.(string); ok {
			v.onImageChanged(img)
		}
	case "internal_ip_type":
		if val, ok := msg.Value.(string); ok {
			if ipField := v.Form.GetField("internal_ip"); ipField != nil {
				if val == "custom" {
					ipField.SetHidden(false)
				} else {
					ipField.SetHidden(true)
					// Clear the value when switching back to auto
					ipField.SetValue("")
				}
			}
		}
	}
	return nil
}

// checkImageChange detects image dropdown changes that weren't caught by FieldChangedMsg
// and updates the default disk size accordingly.
func (v *InstanceCreateView) checkImageChange() {
	if v.Form == nil {
		return
	}
	data := v.Form.GetData()
	if img, ok := data["image"].(string); ok && img != "" && img != v.lastImage {
		v.onImageChanged(img)
	}
}

// onZoneChanged triggers machine type and subnetwork fetching for the selected zone.
func (v *InstanceCreateView) onZoneChanged(zone string) tea.Cmd {
	v.lastZone = zone
	var cmds []tea.Cmd

	// Load machine types (use cache if available, otherwise fetch even if
	// another zone fetch is in flight — its result will be cached but ignored
	// for the dropdown since lastZone won't match)
	if cached, ok := v.machineTypeCache[zone]; ok {
		v.updateMachineTypeDropdown(cached)
	} else {
		v.loadingMachineZone = zone
		if field := v.Form.GetField("machine_type"); field != nil {
			field.SetPlaceholder("Loading...")
		}
		cmds = append(cmds, v.fetchMachineTypes(zone))
	}

	// Load subnetworks and addresses for the zone's region
	region := gcp.RegionFromZone(zone)
	cmds = append(cmds, v.fetchSubnetworks(region))
	cmds = append(cmds, v.fetchAddresses(region))

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
		field.SetPlaceholder("")
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

// filterSubnetsByNetwork updates the subnetwork dropdown to show only subnets
// belonging to the currently selected network.
func (v *InstanceCreateView) filterSubnetsByNetwork() {
	field := v.Form.GetField("subnetwork")
	if field == nil {
		return
	}
	selectedNetwork := ""
	if data := v.Form.GetData(); data != nil {
		if n, ok := data["network"].(string); ok {
			selectedNetwork = n
		}
	}
	// Filter cached subnets by the selected network
	var filtered []gcp.SubnetworkInfo
	for _, s := range v.lastLoadedSubnets {
		if selectedNetwork == "" || s.Network == selectedNetwork {
			filtered = append(filtered, s)
		}
	}
	field.SetOptions(subnetworkDropdownOptions(filtered))
	if len(filtered) > 0 {
		field.SetValue(filtered[0].Name)
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

func (v *InstanceCreateView) fetchAddresses(region string) tea.Cmd {
	return func() tea.Msg {
		if v.computeClient == nil {
			return instanceAddressesLoadedMsg{addresses: nil}
		}
		addresses, err := v.computeClient.ListAddresses(gocontext.Background(), v.projectID, region)
		if err != nil {
			// Non-fatal — keep base ephemeral/none options
			return instanceAddressesLoadedMsg{addresses: nil}
		}
		return instanceAddressesLoadedMsg{addresses: addresses}
	}
}

// handleSubmit validates form data and shows the confirmation view.
func (v *InstanceCreateView) handleSubmit() tea.Cmd {
	if errors := v.Form.Validate(); len(errors) > 0 {
		return nil
	}

	config := v.extractConfig()
	v.pendingConfig = &config
	v.showConfirmation(config)
	return nil
}

// confirmCreate proceeds with the actual API call after user confirms.
func (v *InstanceCreateView) confirmCreate() tea.Cmd {
	if v.pendingConfig == nil {
		return nil
	}
	config := *v.pendingConfig
	v.showConfirm = false
	v.diffViewer = nil
	v.pendingConfig = nil

	cmd := v.BeginSaving()
	return tea.Batch(cmd, func() tea.Msg {
		return CreateInstanceMsg{Config: config}
	})
}

// extractConfig builds an InstanceCreateConfig from form data.
func (v *InstanceCreateView) extractConfig() gcp.InstanceCreateConfig {
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

	// External IP: "ephemeral", "none", or "static:<name>"
	externalIPType := "ephemeral"
	externalIPAddr := ""
	if eip, ok := data["external_ip"].(string); ok && eip != "" {
		externalIPType = eip
	}
	// Resolve static address name to actual IP
	if strings.HasPrefix(externalIPType, "static:") {
		addrName := strings.TrimPrefix(externalIPType, "static:")
		for _, addr := range v.lastLoadedAddresses {
			if addr.Name == addrName {
				externalIPAddr = addr.Address
				break
			}
		}
		externalIPType = addrName // Use the address name as the type identifier
	}

	// Internal IP: empty means auto-assign
	internalIP := ""
	if ipType, ok := data["internal_ip_type"].(string); ok && ipType == "custom" {
		if ip, ok := data["internal_ip"].(string); ok {
			internalIP = strings.TrimSpace(ip)
		}
	}

	return gcp.InstanceCreateConfig{
		Name:           name,
		Zone:           zone,
		MachineType:    machineType,
		ImageProject:   imageProject,
		ImageFamily:    imageFamily,
		DiskSizeGB:     diskSizeGB,
		DiskType:       diskType,
		Network:        network,
		Subnetwork:     subnetwork,
		ExternalIPType: externalIPType,
		ExternalIPAddr: externalIPAddr,
		InternalIP:     internalIP,
	}
}

// showConfirmation builds a diff viewer with the VM configuration summary.
func (v *InstanceCreateView) showConfirmation(config gcp.InstanceCreateConfig) {
	// Resolve human-readable labels for display
	imageLabel := imageLabelFromValue(config.ImageProject + "/" + config.ImageFamily)
	diskTypeLabel := diskTypeLabelFromValue(config.DiskType)

	// External IP display
	var externalIPLabel string
	switch config.ExternalIPType {
	case "none":
		externalIPLabel = "None"
	case "ephemeral", "":
		externalIPLabel = "Ephemeral"
	default:
		// Static address — show name and IP
		externalIPLabel = config.ExternalIPType
		if config.ExternalIPAddr != "" {
			externalIPLabel += " (" + config.ExternalIPAddr + ")"
		}
	}

	subnetLabel := config.Subnetwork
	if subnetLabel == "" {
		subnetLabel = "(auto)"
	}

	internalIPLabel := "Automatic"
	if config.InternalIP != "" {
		internalIPLabel = config.InternalIP
	}

	fields := []diff.Field{
		{Label: "Name", OldValue: "", NewValue: config.Name},
		{Label: "Zone", OldValue: "", NewValue: config.Zone},
		{Label: "Machine Type", OldValue: "", NewValue: config.MachineType},
		{Label: "Boot Image", OldValue: "", NewValue: imageLabel},
		{Label: "Boot Disk Size", OldValue: "", NewValue: fmt.Sprintf("%d GB", config.DiskSizeGB)},
		{Label: "Boot Disk Type", OldValue: "", NewValue: diskTypeLabel},
		{Label: "Network", OldValue: "", NewValue: config.Network},
		{Label: "Subnetwork", OldValue: "", NewValue: subnetLabel},
		{Label: "Internal IP", OldValue: "", NewValue: internalIPLabel},
		{Label: "External IP", OldValue: "", NewValue: externalIPLabel},
	}

	v.diffViewer = diff.New("Confirm VM Instance Creation", fields)
	if v.Width > 0 {
		v.diffViewer.SetSize(v.Width-8, v.Height-10)
	}
	v.showConfirm = true
}

// GetComputeClient returns the compute client for reuse
func (v *InstanceCreateView) GetComputeClient() *gcp.ComputeClient {
	return v.computeClient
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
