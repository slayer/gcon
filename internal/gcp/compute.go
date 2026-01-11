package gcp

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

// Instance represents a simplified Compute Engine VM instance
type Instance struct {
	Name        string
	Zone        string
	MachineType string
	Status      string
	InternalIP  string
	ExternalIP  string
	CreatedAt   string
}

// InstanceDetails contains comprehensive VM instance information
type InstanceDetails struct {
	// Basic Information
	Name               string
	ID                 uint64
	Description        string
	Status             string
	Zone               string
	CreatedAt          string
	DeletionProtection bool
	Labels             map[string]string
	Tags               []string

	// Machine Configuration
	MachineType    string
	MachineTypeURI string // Full URI for parsing vCPUs/memory from custom types
	CpuPlatform    string
	MinCpuPlatform string
	DisplayDevice  bool
	GPUs           []GPU

	// Networking
	CanIPForward      bool
	NetworkInterfaces []NetworkInterfaceInfo

	// Storage
	Disks []DiskInfo

	// Security
	ShieldedVM     ShieldedVMConfig
	ServiceAccount string
	Scopes         []string

	// Availability Policies
	Scheduling SchedulingInfo

	// Metadata
	Metadata map[string]string
}

// GPU represents an attached accelerator
type GPU struct {
	Type  string
	Count int64
}

// NetworkInterfaceInfo contains network interface details
type NetworkInterfaceInfo struct {
	Name       string
	Network    string
	Subnetwork string
	NicType    string
	InternalIP string
	ExternalIP string
	StackType  string
	Tier       string
}

// DiskInfo contains attached disk details
type DiskInfo struct {
	Name       string
	SizeGB     int64
	Type       string
	Mode       string
	Boot       bool
	AutoDelete bool
	DeviceName string
	Source     string
}

// ShieldedVMConfig contains Shielded VM settings
type ShieldedVMConfig struct {
	SecureBoot          bool
	VTPM                bool
	IntegrityMonitoring bool
}

// SchedulingInfo contains availability policy settings
type SchedulingInfo struct {
	ProvisioningModel         string
	Preemptible               bool
	OnHostMaintenance         string
	AutomaticRestart          bool
	InstanceTerminationAction string
}

// ComputeClient handles Compute Engine operations
type ComputeClient struct {
	service *compute.Service
}

// NewComputeClient creates a new Compute Engine client
func NewComputeClient(ctx context.Context) (*ComputeClient, error) {
	service, err := compute.NewService(ctx, option.WithScopes(
		compute.ComputeScope,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}

	return &ComputeClient{service: service}, nil
}

// ListInstances returns all VM instances across all zones in a project
func (c *ComputeClient) ListInstances(ctx context.Context, projectID string) ([]Instance, error) {
	var instances []Instance

	// Use aggregatedList to get instances from all zones in one call
	req := c.service.Instances.AggregatedList(projectID)
	err := req.Pages(ctx, func(page *compute.InstanceAggregatedList) error {
		for zone, scopedList := range page.Items {
			if scopedList.Instances == nil {
				continue
			}
			for _, inst := range scopedList.Instances {
				instances = append(instances, instanceFromAPI(inst, zone))
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	return instances, nil
}

// ListInstancesInZone returns VM instances in a specific zone
func (c *ComputeClient) ListInstancesInZone(ctx context.Context, projectID, zone string) ([]Instance, error) {
	var instances []Instance

	req := c.service.Instances.List(projectID, zone)
	err := req.Pages(ctx, func(page *compute.InstanceList) error {
		for _, inst := range page.Items {
			instances = append(instances, instanceFromAPI(inst, zone))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list instances in zone %s: %w", zone, err)
	}

	return instances, nil
}

// StartInstance starts a stopped VM instance
func (c *ComputeClient) StartInstance(ctx context.Context, projectID, zone, instanceName string) error {
	_, err := c.service.Instances.Start(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to start instance %s: %w", instanceName, err)
	}
	return nil
}

// StopInstance stops a running VM instance
func (c *ComputeClient) StopInstance(ctx context.Context, projectID, zone, instanceName string) error {
	_, err := c.service.Instances.Stop(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to stop instance %s: %w", instanceName, err)
	}
	return nil
}

// ResetInstance resets (hard reboot) a VM instance
func (c *ComputeClient) ResetInstance(ctx context.Context, projectID, zone, instanceName string) error {
	_, err := c.service.Instances.Reset(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to reset instance %s: %w", instanceName, err)
	}
	return nil
}

// GetInstance returns details for a specific instance
func (c *ComputeClient) GetInstance(ctx context.Context, projectID, zone, instanceName string) (*Instance, error) {
	inst, err := c.service.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance %s: %w", instanceName, err)
	}

	result := instanceFromAPI(inst, zone)
	return &result, nil
}

// GetInstanceDetails returns comprehensive details for a specific instance
func (c *ComputeClient) GetInstanceDetails(ctx context.Context, projectID, zone, instanceName string) (*InstanceDetails, error) {
	inst, err := c.service.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get instance details %s: %w", instanceName, err)
	}

	return instanceDetailsFromAPI(inst, zone), nil
}

// instanceDetailsFromAPI converts full API instance to InstanceDetails struct
func instanceDetailsFromAPI(inst *compute.Instance, zone string) *InstanceDetails {
	details := &InstanceDetails{
		Name:               inst.Name,
		ID:                 inst.Id,
		Description:        inst.Description,
		Status:             inst.Status,
		Zone:               extractName(zone),
		CreatedAt:          inst.CreationTimestamp,
		DeletionProtection: inst.DeletionProtection,
		Labels:             inst.Labels,
		MachineType:        extractName(inst.MachineType),
		MachineTypeURI:     inst.MachineType,
		CpuPlatform:        inst.CpuPlatform,
		MinCpuPlatform:     inst.MinCpuPlatform,
		CanIPForward:       inst.CanIpForward,
	}

	// Tags
	if inst.Tags != nil {
		details.Tags = inst.Tags.Items
	}

	// Display device
	if inst.DisplayDevice != nil {
		details.DisplayDevice = inst.DisplayDevice.EnableDisplay
	}

	// GPUs
	for _, acc := range inst.GuestAccelerators {
		details.GPUs = append(details.GPUs, GPU{
			Type:  extractName(acc.AcceleratorType),
			Count: acc.AcceleratorCount,
		})
	}

	// Network interfaces
	for _, nic := range inst.NetworkInterfaces {
		ni := NetworkInterfaceInfo{
			Name:       nic.Name,
			Network:    extractName(nic.Network),
			Subnetwork: extractName(nic.Subnetwork),
			NicType:    nic.NicType,
			InternalIP: nic.NetworkIP,
			StackType:  nic.StackType,
		}
		// External IP from access configs
		if len(nic.AccessConfigs) > 0 {
			ni.ExternalIP = nic.AccessConfigs[0].NatIP
			ni.Tier = nic.AccessConfigs[0].NetworkTier
		}
		details.NetworkInterfaces = append(details.NetworkInterfaces, ni)
	}

	// Disks
	for _, disk := range inst.Disks {
		d := DiskInfo{
			Name:       extractName(disk.Source),
			SizeGB:     disk.DiskSizeGb,
			Mode:       disk.Mode,
			Boot:       disk.Boot,
			AutoDelete: disk.AutoDelete,
			DeviceName: disk.DeviceName,
			Source:     disk.Source,
		}
		// Extract disk type from source URL or interface
		if disk.Interface != "" {
			d.Type = disk.Interface
		}
		details.Disks = append(details.Disks, d)
	}

	// Shielded VM config
	if inst.ShieldedInstanceConfig != nil {
		details.ShieldedVM = ShieldedVMConfig{
			SecureBoot:          inst.ShieldedInstanceConfig.EnableSecureBoot,
			VTPM:                inst.ShieldedInstanceConfig.EnableVtpm,
			IntegrityMonitoring: inst.ShieldedInstanceConfig.EnableIntegrityMonitoring,
		}
	}

	// Service account
	if len(inst.ServiceAccounts) > 0 {
		details.ServiceAccount = inst.ServiceAccounts[0].Email
		details.Scopes = inst.ServiceAccounts[0].Scopes
	}

	// Scheduling
	if inst.Scheduling != nil {
		details.Scheduling = SchedulingInfo{
			ProvisioningModel:         inst.Scheduling.ProvisioningModel,
			Preemptible:               inst.Scheduling.Preemptible,
			OnHostMaintenance:         inst.Scheduling.OnHostMaintenance,
			InstanceTerminationAction: inst.Scheduling.InstanceTerminationAction,
		}
		if inst.Scheduling.AutomaticRestart != nil {
			details.Scheduling.AutomaticRestart = *inst.Scheduling.AutomaticRestart
		}
	}

	// Metadata
	if inst.Metadata != nil && len(inst.Metadata.Items) > 0 {
		details.Metadata = make(map[string]string)
		for _, item := range inst.Metadata.Items {
			if item.Value != nil {
				details.Metadata[item.Key] = *item.Value
			} else {
				details.Metadata[item.Key] = ""
			}
		}
	}

	return details
}

// extractName extracts the last component from a GCP resource path
func extractName(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

// instanceFromAPI converts API instance to our simplified struct
func instanceFromAPI(inst *compute.Instance, zone string) Instance {
	// Extract zone name from full path (zones/us-central1-a -> us-central1-a)
	zoneName := zone
	if parts := strings.Split(zone, "/"); len(parts) > 0 {
		zoneName = parts[len(parts)-1]
	}

	// Extract machine type name from full path
	machineType := inst.MachineType
	if parts := strings.Split(inst.MachineType, "/"); len(parts) > 0 {
		machineType = parts[len(parts)-1]
	}

	// Get internal IP
	var internalIP string
	if len(inst.NetworkInterfaces) > 0 {
		internalIP = inst.NetworkInterfaces[0].NetworkIP
	}

	// Get external IP (if exists)
	var externalIP string
	if len(inst.NetworkInterfaces) > 0 && len(inst.NetworkInterfaces[0].AccessConfigs) > 0 {
		externalIP = inst.NetworkInterfaces[0].AccessConfigs[0].NatIP
	}

	return Instance{
		Name:        inst.Name,
		Zone:        zoneName,
		MachineType: machineType,
		Status:      inst.Status,
		InternalIP:  internalIP,
		ExternalIP:  externalIP,
		CreatedAt:   inst.CreationTimestamp,
	}
}

// ZoneFromInstance extracts zone name for API calls
func (i *Instance) ZoneFromInstance() string {
	return i.Zone
}

// IsRunning returns true if the instance is in RUNNING state
func (i *Instance) IsRunning() bool {
	return i.Status == "RUNNING"
}

// IsStopped returns true if the instance is in TERMINATED or STOPPED state
func (i *Instance) IsStopped() bool {
	return i.Status == "TERMINATED" || i.Status == "STOPPED"
}
