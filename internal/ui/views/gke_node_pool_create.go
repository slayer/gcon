package views

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	container "google.golang.org/api/container/v1"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/forms"
)

// errNodePoolInitialCountTooLow is returned when initial_count < autoscaling min_nodes.
var errNodePoolInitialCountTooLow = errors.New("initial count must be >= min_nodes when autoscaling is enabled")

// errNodePoolAutoscaleRangeInverted is returned when min_nodes > max_nodes.
var errNodePoolAutoscaleRangeInverted = errors.New("min_nodes must be <= max_nodes")

// GKENodePoolCreateView is a form-based creation view for new GKE node pools.
type GKENodePoolCreateView struct {
	CreateViewBase
	projectID, location, clusterName string
	compute                          *gcp.ComputeClient // optional, for machine-type dropdown
}

// NewGKENodePoolCreateView creates a new node pool creation view.
func NewGKENodePoolCreateView(projectID, location, clusterName string, compute *gcp.ComputeClient) *GKENodePoolCreateView {
	v := &GKENodePoolCreateView{
		CreateViewBase: NewCreateViewBase("Creating node pool..."),
		projectID:      projectID,
		location:       location,
		clusterName:    clusterName,
		compute:        compute,
	}
	v.buildForm()
	return v
}

// buildForm assembles the node pool creation form with Basic, Autoscaling, and Lifecycle sections.
func (v *GKENodePoolCreateView) buildForm() {
	v.Form = forms.NewForm(
		fmt.Sprintf("Create Node Pool in %s", v.clusterName),
		forms.FormModeCreate,
	).EnableViewport()

	// Basic section
	basicSection := forms.NewSection("basic", "Basic").
		AddField(forms.NewTextField("name", "Pool Name").
			SetRequired(true).
			SetPlaceholder("default-pool").
			SetHelpText("1-63 characters, lowercase letters, numbers, and hyphens").
			SetValidator(forms.ComposeValidators(
				forms.ValidateRequired,
				forms.ValidateGCPResourceName,
			))).
		AddField(forms.NewNumberField("initial_count", "Initial Node Count").
			SetValue(int64(1)).
			SetHelpText("Number of nodes to create initially").
			SetValidator(forms.ValidateNumber(1, 1000))).
		AddField(forms.NewDropdownField("machine_type", "Machine Type").
			SetOptions([]forms.Option{
				{Value: "e2-medium", Label: "e2-medium (2 vCPU, 4 GB)"},
				{Value: "e2-standard-2", Label: "e2-standard-2 (2 vCPU, 8 GB)"},
				{Value: "e2-standard-4", Label: "e2-standard-4 (4 vCPU, 16 GB)"},
				{Value: "n2-standard-2", Label: "n2-standard-2 (2 vCPU, 8 GB)"},
				{Value: "n2-standard-4", Label: "n2-standard-4 (4 vCPU, 16 GB)"},
			}).
			SetHelpText("VM machine type for nodes")).
		AddField(forms.NewDropdownField("disk_type", "Disk Type").
			SetOptions([]forms.Option{
				{Value: "pd-balanced", Label: "Balanced persistent disk"},
				{Value: "pd-standard", Label: "Standard persistent disk"},
				{Value: "pd-ssd", Label: "SSD persistent disk"},
			}).
			SetHelpText("Boot disk type for nodes")).
		AddField(forms.NewNumberField("disk_size_gb", "Disk Size (GB)").
			SetValue(int64(100)).
			SetHelpText("Boot disk size in GB").
			SetValidator(forms.ValidateNumber(10, 65536))).
		AddField(forms.NewDropdownField("image_type", "Image Type").
			SetOptions([]forms.Option{
				{Value: "COS_CONTAINERD", Label: "Container-Optimized OS (containerd)"},
				{Value: "UBUNTU_CONTAINERD", Label: "Ubuntu (containerd)"},
			}).
			SetHelpText("Node OS image type"))

	v.Form.AddSection(basicSection)

	// Autoscaling section
	autoscalingSection := forms.NewSection("autoscaling", "Autoscaling").
		AddField(forms.NewToggleField("autoscale_enabled", "Enable Autoscaling").
			SetHelpText("Automatically scale the node pool based on workload")).
		AddField(forms.NewNumberField("min_nodes", "Minimum Nodes").
			SetValue(int64(1)).
			SetHelpText("Minimum number of nodes when autoscaling is enabled").
			SetValidator(forms.ValidateNumber(0, 1000))).
		AddField(forms.NewNumberField("max_nodes", "Maximum Nodes").
			SetValue(int64(3)).
			SetHelpText("Maximum number of nodes when autoscaling is enabled").
			SetValidator(forms.ValidateNumber(1, 1000)))

	v.Form.AddSection(autoscalingSection)

	// Lifecycle section
	lifecycleSection := forms.NewSection("lifecycle", "Lifecycle").
		AddField(forms.NewToggleField("auto_upgrade", "Auto-upgrade").
			SetValue(true).
			SetHelpText("Automatically upgrade nodes to newer Kubernetes versions")).
		AddField(forms.NewToggleField("auto_repair", "Auto-repair").
			SetValue(true).
			SetHelpText("Automatically repair unhealthy nodes")).
		AddField(forms.NewToggleField("preemptible", "Preemptible Nodes").
			SetHelpText("Use preemptible VMs for lower cost (may be interrupted)"))

	v.Form.AddSection(lifecycleSection)
}

// Update handles messages for the node pool create view.
func (v *GKENodePoolCreateView) Update(msg tea.Msg) tea.Cmd {
	// Let base handle spinner ticks and cancel-during-saving
	if cmd, handled := v.HandleBaseUpdate(msg, GKENodePoolCreateCanceledMsg{}); handled {
		return cmd
	}

	switch msg.(type) {
	case forms.FormSubmitMsg:
		return v.handleSubmit()
	case forms.FormCancelMsg:
		return func() tea.Msg { return GKENodePoolCreateCanceledMsg{} }
	}

	return v.UpdateForm(msg)
}

// handleSubmit validates the form and emits GKENodePoolCreateRequestMsg.
func (v *GKENodePoolCreateView) handleSubmit() tea.Cmd {
	if errs := v.Form.Validate(); len(errs) > 0 {
		return nil
	}
	data := v.Form.GetData()
	pool := buildNodePoolFromForm(data)

	// Cross-field checks for autoscaling: range must be sane AND initial count
	// must be high enough to land inside it. min > max is checked first so the
	// user fixes the range before learning the initial-count constraint.
	if pool.Autoscaling != nil && pool.Autoscaling.Enabled {
		if pool.Autoscaling.MinNodeCount > pool.Autoscaling.MaxNodeCount {
			v.SetError(fmt.Errorf("%w: min=%d, max=%d", errNodePoolAutoscaleRangeInverted, pool.Autoscaling.MinNodeCount, pool.Autoscaling.MaxNodeCount))
			return nil
		}
		if pool.InitialNodeCount < pool.Autoscaling.MinNodeCount {
			v.SetError(fmt.Errorf("%w: initial=%d, min=%d", errNodePoolInitialCountTooLow, pool.InitialNodeCount, pool.Autoscaling.MinNodeCount))
			return nil
		}
	}

	// Transition to saving state; BeginSaving returns the first spinner tick cmd.
	beginCmd := v.BeginSaving()
	return tea.Batch(beginCmd, func() tea.Msg {
		return GKENodePoolCreateRequestMsg{
			ProjectID:   v.projectID,
			Location:    v.location,
			ClusterName: v.clusterName,
			Pool:        pool,
		}
	})
}

// buildNodePoolFromForm constructs a container.NodePool from form data.
func buildNodePoolFromForm(data map[string]any) *container.NodePool {
	var (
		name         string
		initialCount int64
		machineType  string
		diskType     string
		diskSize     int64
		imageType    string

		autoscaleEnabled bool
		minNodes         int64
		maxNodes         int64

		autoUpgrade bool
		autoRepair  bool
		preemptible bool
	)

	if v, ok := data["name"].(string); ok {
		name = v
	}
	if v, ok := data["initial_count"].(int64); ok {
		initialCount = v
	}
	if v, ok := data["machine_type"].(string); ok {
		machineType = v
	}
	if v, ok := data["disk_type"].(string); ok {
		diskType = v
	}
	if v, ok := data["disk_size_gb"].(int64); ok {
		diskSize = v
	}
	if v, ok := data["image_type"].(string); ok {
		imageType = v
	}
	if v, ok := data["autoscale_enabled"].(bool); ok {
		autoscaleEnabled = v
	}
	if v, ok := data["min_nodes"].(int64); ok {
		minNodes = v
	}
	if v, ok := data["max_nodes"].(int64); ok {
		maxNodes = v
	}
	if v, ok := data["auto_upgrade"].(bool); ok {
		autoUpgrade = v
	}
	if v, ok := data["auto_repair"].(bool); ok {
		autoRepair = v
	}
	if v, ok := data["preemptible"].(bool); ok {
		preemptible = v
	}

	pool := &container.NodePool{
		Name:             name,
		InitialNodeCount: initialCount,
		Config: &container.NodeConfig{
			MachineType: machineType,
			DiskType:    diskType,
			DiskSizeGb:  diskSize,
			ImageType:   imageType,
			Preemptible: preemptible,
		},
		Management: &container.NodeManagement{
			AutoUpgrade: autoUpgrade,
			AutoRepair:  autoRepair,
		},
	}
	if autoscaleEnabled {
		pool.Autoscaling = &container.NodePoolAutoscaling{
			Enabled:      true,
			MinNodeCount: minNodes,
			MaxNodeCount: maxNodes,
		}
	}
	return pool
}
