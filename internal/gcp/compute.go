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

// Disk represents a simplified Compute Engine persistent disk
type Disk struct {
	Name       string
	Zone       string
	SizeGB     int64
	Type       string // pd-standard, pd-ssd, pd-balanced, etc.
	Status     string // READY, CREATING, FAILED, etc.
	AttachedTo string // Instance name if attached, empty otherwise
	CreatedAt  string
}

// DiskDetails contains comprehensive persistent disk information
type DiskDetails struct {
	// Basic Information
	Name        string
	ID          uint64
	Description string
	Status      string
	Zone        string
	CreatedAt   string
	LastAttach  string
	LastDetach  string
	Labels      map[string]string

	// Size and Type
	SizeGB          int64
	Type            string // pd-standard, pd-ssd, pd-balanced, pd-extreme
	ProvisionedIOPS int64  // For pd-extreme disks
	ProvisionedTPUT int64  // Provisioned throughput in MB/s

	// Source
	SourceImage    string
	SourceSnapshot string
	SourceDisk     string

	// Encryption
	DiskEncryptionKey string // Type of encryption (Google-managed, CMEK, etc.)

	// Usage
	Users              []string // Instances using this disk
	ReplicaZones       []string // For regional disks
	PhysicalBlockSizeB int64    // Physical block size in bytes
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
		return nil, WrapListError(err, "instances", projectID)
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
		return nil, WrapListError(err, "instances", zone)
	}

	return instances, nil
}

// StartInstance starts a stopped VM instance
func (c *ComputeClient) StartInstance(ctx context.Context, projectID, zone, instanceName string) error {
	_, err := c.service.Instances.Start(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "start instance", instanceName)
	}
	return nil
}

// StopInstance stops a running VM instance
func (c *ComputeClient) StopInstance(ctx context.Context, projectID, zone, instanceName string) error {
	_, err := c.service.Instances.Stop(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "stop instance", instanceName)
	}
	return nil
}

// ResetInstance resets (hard reboot) a VM instance
func (c *ComputeClient) ResetInstance(ctx context.Context, projectID, zone, instanceName string) error {
	_, err := c.service.Instances.Reset(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "reset instance", instanceName)
	}
	return nil
}

// GetInstance returns details for a specific instance
func (c *ComputeClient) GetInstance(ctx context.Context, projectID, zone, instanceName string) (*Instance, error) {
	inst, err := c.service.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "instance", instanceName)
	}

	result := instanceFromAPI(inst, zone)
	return &result, nil
}

// GetInstanceDetails returns comprehensive details for a specific instance
func (c *ComputeClient) GetInstanceDetails(ctx context.Context, projectID, zone, instanceName string) (*InstanceDetails, error) {
	inst, err := c.service.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "instance details", instanceName)
	}

	return instanceDetailsFromAPI(inst, zone), nil
}

// ListDisks returns all persistent disks across all zones in a project
func (c *ComputeClient) ListDisks(ctx context.Context, projectID string) ([]Disk, error) {
	var disks []Disk

	// Use aggregatedList to get disks from all zones in one call
	req := c.service.Disks.AggregatedList(projectID)
	err := req.Pages(ctx, func(page *compute.DiskAggregatedList) error {
		for zone, scopedList := range page.Items {
			if scopedList.Disks == nil {
				continue
			}
			for _, d := range scopedList.Disks {
				disks = append(disks, diskFromAPI(d, zone))
			}
		}
		return nil
	})
	if err != nil {
		return nil, WrapListError(err, "disks", projectID)
	}

	return disks, nil
}

// diskFromAPI converts API disk to our simplified struct
func diskFromAPI(d *compute.Disk, zone string) Disk {
	// Extract zone name from full path (zones/us-central1-a -> us-central1-a)
	zoneName := extractName(zone)

	// Extract disk type name from full path
	diskType := extractName(d.Type)

	// Get attached instance name if any
	var attachedTo string
	if len(d.Users) > 0 {
		// Users field contains full instance URLs, extract instance name
		attachedTo = extractName(d.Users[0])
	}

	return Disk{
		Name:       d.Name,
		Zone:       zoneName,
		SizeGB:     d.SizeGb,
		Type:       diskType,
		Status:     d.Status,
		AttachedTo: attachedTo,
		CreatedAt:  d.CreationTimestamp,
	}
}

// IsAttached returns true if the disk is attached to an instance
func (d *Disk) IsAttached() bool {
	return d.AttachedTo != ""
}

// IsReady returns true if the disk is in READY state
func (d *Disk) IsReady() bool {
	return d.Status == "READY"
}

// GetDiskDetails returns comprehensive details for a specific disk
func (c *ComputeClient) GetDiskDetails(ctx context.Context, projectID, zone, diskName string) (*DiskDetails, error) {
	disk, err := c.service.Disks.Get(projectID, zone, diskName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "disk details", diskName)
	}

	return diskDetailsFromAPI(disk, zone), nil
}

// diskDetailsFromAPI converts full API disk to DiskDetails struct
func diskDetailsFromAPI(d *compute.Disk, zone string) *DiskDetails {
	details := &DiskDetails{
		Name:               d.Name,
		ID:                 d.Id,
		Description:        d.Description,
		Status:             d.Status,
		Zone:               extractName(zone),
		CreatedAt:          d.CreationTimestamp,
		LastAttach:         d.LastAttachTimestamp,
		LastDetach:         d.LastDetachTimestamp,
		Labels:             d.Labels,
		SizeGB:             d.SizeGb,
		Type:               extractName(d.Type),
		ProvisionedIOPS:    d.ProvisionedIops,
		ProvisionedTPUT:    d.ProvisionedThroughput,
		SourceImage:        extractName(d.SourceImage),
		SourceSnapshot:     extractName(d.SourceSnapshot),
		SourceDisk:         extractName(d.SourceDisk),
		PhysicalBlockSizeB: d.PhysicalBlockSizeBytes,
	}

	// Extract user instance names from full URLs
	for _, user := range d.Users {
		details.Users = append(details.Users, extractName(user))
	}

	// Extract replica zones for regional disks
	for _, rz := range d.ReplicaZones {
		details.ReplicaZones = append(details.ReplicaZones, extractName(rz))
	}

	// Determine encryption type
	details.DiskEncryptionKey = "Google-managed"
	if d.DiskEncryptionKey != nil {
		switch {
		case d.DiskEncryptionKey.KmsKeyName != "":
			details.DiskEncryptionKey = "Customer-managed (CMEK)"
		case d.DiskEncryptionKey.RawKey != "":
			details.DiskEncryptionKey = "Customer-supplied (CSEK)"
		}
	}

	return details
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

// GetInstanceMetadata retrieves metadata for a specific instance
func (c *ComputeClient) GetInstanceMetadata(ctx context.Context, projectID, zone, instanceName string) (*InstanceMetadata, error) {
	inst, err := c.service.Instances.Get(projectID, zone, instanceName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "instance metadata", instanceName)
	}

	metadata := &InstanceMetadata{
		Items:       make(map[string]string),
		Fingerprint: "",
	}

	if inst.Metadata != nil {
		metadata.Fingerprint = inst.Metadata.Fingerprint
		for _, item := range inst.Metadata.Items {
			if item.Value != nil {
				metadata.Items[item.Key] = *item.Value
			} else {
				metadata.Items[item.Key] = ""
			}
		}
	}

	return metadata, nil
}

// SetInstanceMetadata updates metadata for a specific instance
// Uses fingerprint for optimistic locking to prevent concurrent updates
func (c *ComputeClient) SetInstanceMetadata(ctx context.Context, projectID, zone, instanceName string, metadata map[string]string, fingerprint string) error {
	// Convert map to API format
	var items []*compute.MetadataItems
	for key, value := range metadata {
		valueCopy := value // Create a copy for the pointer
		items = append(items, &compute.MetadataItems{
			Key:   key,
			Value: &valueCopy,
		})
	}

	metadataObj := &compute.Metadata{
		Fingerprint: fingerprint,
		Items:       items,
	}

	_, err := c.service.Instances.SetMetadata(projectID, zone, instanceName, metadataObj).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "set instance metadata", instanceName)
	}

	return nil
}

// GetProjectMetadata retrieves project-level metadata with fingerprint
// This metadata applies to all instances in the project (commonInstanceMetadata)
func (c *ComputeClient) GetProjectMetadata(ctx context.Context, projectID string) (*InstanceMetadata, error) {
	project, err := c.service.Projects.Get(projectID).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "project metadata", projectID)
	}

	metadata := &InstanceMetadata{
		Items:       make(map[string]string),
		Fingerprint: "",
	}

	if project.CommonInstanceMetadata != nil {
		metadata.Fingerprint = project.CommonInstanceMetadata.Fingerprint
		for _, item := range project.CommonInstanceMetadata.Items {
			if item.Value != nil {
				metadata.Items[item.Key] = *item.Value
			} else {
				metadata.Items[item.Key] = ""
			}
		}
	}

	return metadata, nil
}

// SetProjectMetadata updates project-wide metadata (commonInstanceMetadata)
// This metadata is inherited by all instances in the project
// Uses fingerprint for optimistic locking to prevent concurrent updates
func (c *ComputeClient) SetProjectMetadata(ctx context.Context, projectID string, metadata map[string]string, fingerprint string) error {
	// Convert map to API format
	var items []*compute.MetadataItems
	for key, value := range metadata {
		valueCopy := value
		items = append(items, &compute.MetadataItems{
			Key:   key,
			Value: &valueCopy,
		})
	}

	metadataObj := &compute.Metadata{
		Fingerprint: fingerprint,
		Items:       items,
	}

	_, err := c.service.Projects.SetCommonInstanceMetadata(projectID, metadataObj).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "set project metadata", projectID)
	}

	return nil
}
