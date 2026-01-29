package gcp

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

// Storage location options for snapshots and images
var (
	// ComputeMultiRegions contains multi-regional storage locations
	ComputeMultiRegions = []string{"us", "eu", "asia"}

	// ComputeRegions contains regional storage locations (same as GCS regions)
	ComputeRegions = []string{
		"us-central1", "us-east1", "us-east4", "us-east5", "us-south1", "us-west1", "us-west2", "us-west3", "us-west4",
		"northamerica-northeast1", "northamerica-northeast2", "southamerica-east1", "southamerica-west1",
		"europe-central2", "europe-north1", "europe-southwest1", "europe-west1", "europe-west2", "europe-west3",
		"europe-west4", "europe-west6", "europe-west8", "europe-west9", "europe-west10", "europe-west12",
		"asia-east1", "asia-east2", "asia-northeast1", "asia-northeast2", "asia-northeast3",
		"asia-south1", "asia-south2", "asia-southeast1", "asia-southeast2",
		"australia-southeast1", "australia-southeast2",
		"me-central1", "me-central2", "me-west1",
		"africa-south1",
	}

	// AllStorageLocations contains all storage locations (multi-regional + regional)
	// Use explicit copy to avoid modifying ComputeMultiRegions' underlying array
	AllStorageLocations = append(append([]string{}, ComputeMultiRegions...), ComputeRegions...)
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

// Snapshot represents a simplified disk snapshot
type Snapshot struct {
	Name             string
	SourceDisk       string // Name of the source disk
	SourceDiskID     string // Full ID/URI of source disk
	SizeGB           int64
	Status           string // CREATING, UPLOADING, READY, FAILED, DELETING
	CreatedAt        string
	StorageBytes     int64    // Actual storage used (may be less than SizeGB due to compression)
	StorageLocations []string // Storage locations (regions)
}

// SnapshotDetails contains comprehensive snapshot information
type SnapshotDetails struct {
	// Basic Information
	Name        string
	ID          uint64
	Description string
	Status      string
	CreatedAt   string
	Labels      map[string]string

	// Source
	SourceDisk     string // Name of source disk
	SourceDiskID   string // Full disk URL
	SourceDiskZone string // Zone where source disk was located

	// Size and Storage
	SizeGB            int64 // Size of source disk
	StorageBytes      int64 // Actual storage used
	StorageBytesGb    int64 // Storage in GB (StorageBytes / 1024^3)
	DiskSizeGB        int64 // Original disk size
	StorageLocations  []string
	DownloadBytes     int64
	AutoCreated       bool
	ChainName         string
	SatisfiesPZS      bool
	SnapshotType      string
	CreationSizeBytes int64

	// Encryption
	SnapshotEncryptionKey string // Type of encryption
	SourceDiskEncryption  string // Encryption of source disk
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

// Image represents a simplified Compute Engine disk image
type Image struct {
	Name             string
	Family           string
	Status           string // READY, FAILED, PENDING, DELETING
	DiskSizeGB       int64
	ArchiveSizeBytes int64
	SourceType       string // RAW, etc.
	CreatedAt        string
	CreatedBy        string   // Source project or system (e.g., "debian-cloud", project ID)
	StorageLocations []string // Regional or multi-regional locations
	Architecture     string   // ARM64 or X86_64
}

// ImageDetails contains comprehensive disk image information
type ImageDetails struct {
	// Basic Information
	Name         string
	ID           uint64
	Description  string
	Family       string
	Status       string
	CreatedAt    string
	CreatedBy    string // Source project or system
	Architecture string // ARM64 or X86_64
	Deprecated   *DeprecationStatus
	Labels       map[string]string

	// Size Information
	DiskSizeGB       int64
	ArchiveSizeB     int64
	StorageBytes     int64
	StorageLocations []string

	// Source
	SourceType     string // RAW, etc.
	SourceDisk     string
	SourceDiskID   string
	SourceSnapshot string
	SourceImage    string
	SourceImageID  string

	// Image Features
	GuestOSFeatures           []string
	Licenses                  []string
	EnableConfidentialCompute bool
	SatisfiesPzs              bool // Physical zone separation
	LicenseCodes              []int64

	// Encryption
	ImageEncryptionKey          string
	SourceDiskEncryptionKey     string
	SourceSnapshotEncryptionKey string
}

// DeprecationStatus holds deprecation information for an image
type DeprecationStatus struct {
	State       string // DEPRECATED, OBSOLETE, DELETED
	Replacement string
	Deprecated  string // timestamp
	Obsolete    string // timestamp
	Deleted     string // timestamp
}

// IsReady returns true if the image is in READY state
func (img *Image) IsReady() bool {
	return img.Status == "READY"
}

// ListImages returns all custom images in a project
func (c *ComputeClient) ListImages(ctx context.Context, projectID string) ([]Image, error) {
	var images []Image

	req := c.service.Images.List(projectID)
	err := req.Pages(ctx, func(page *compute.ImageList) error {
		for _, img := range page.Items {
			images = append(images, imageFromAPI(img))
		}
		return nil
	})
	if err != nil {
		return nil, WrapListError(err, "images", projectID)
	}

	return images, nil
}

// GetImageDetails returns comprehensive details for a specific image
func (c *ComputeClient) GetImageDetails(ctx context.Context, projectID, imageName string) (*ImageDetails, error) {
	img, err := c.service.Images.Get(projectID, imageName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "image details", imageName)
	}

	return imageDetailsFromAPI(img), nil
}

// DeleteImage deletes a disk image
func (c *ComputeClient) DeleteImage(ctx context.Context, projectID, imageName string) error {
	_, err := c.service.Images.Delete(projectID, imageName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "delete image", imageName)
	}
	return nil
}

// ListSnapshots returns all snapshots in a project
func (c *ComputeClient) ListSnapshots(ctx context.Context, projectID string) ([]Snapshot, error) {
	var snapshots []Snapshot

	req := c.service.Snapshots.List(projectID)
	err := req.Pages(ctx, func(page *compute.SnapshotList) error {
		for _, s := range page.Items {
			snapshots = append(snapshots, snapshotFromAPI(s))
		}
		return nil
	})
	if err != nil {
		return nil, WrapListError(err, "snapshots", projectID)
	}

	return snapshots, nil
}

// GetSnapshotDetails returns comprehensive details for a specific snapshot
func (c *ComputeClient) GetSnapshotDetails(ctx context.Context, projectID, snapshotName string) (*SnapshotDetails, error) {
	snapshot, err := c.service.Snapshots.Get(projectID, snapshotName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "snapshot details", snapshotName)
	}

	return snapshotDetailsFromAPI(snapshot), nil
}

// DeleteSnapshot deletes a snapshot
func (c *ComputeClient) DeleteSnapshot(ctx context.Context, projectID, snapshotName string) error {
	_, err := c.service.Snapshots.Delete(projectID, snapshotName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "delete snapshot", snapshotName)
	}
	return nil
}

// imageFromAPI converts API image to our simplified struct
func imageFromAPI(img *compute.Image) Image {
	family := img.Family
	if family == "" {
		family = "-"
	}

	// Extract creator/source project from self link or source disk
	createdBy := extractCreatorFromImage(img)

	// Get first storage location if available
	location := "-"
	if len(img.StorageLocations) > 0 {
		location = img.StorageLocations[0]
	}

	arch := img.Architecture
	if arch == "" {
		arch = "X86_64" // Default
	}

	return Image{
		Name:             img.Name,
		Family:           family,
		Status:           img.Status,
		DiskSizeGB:       img.DiskSizeGb,
		ArchiveSizeBytes: img.ArchiveSizeBytes,
		SourceType:       img.SourceType,
		CreatedAt:        img.CreationTimestamp,
		CreatedBy:        createdBy,
		StorageLocations: []string{location},
		Architecture:     arch,
	}
}

// extractCreatorFromImage extracts the creator/source project from image
func extractCreatorFromImage(img *compute.Image) string {
	// Try to extract from self link first
	if img.SelfLink != "" {
		parts := strings.Split(img.SelfLink, "/")
		for i, part := range parts {
			if part == "projects" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}

	// If source disk exists, try to extract from that
	if img.SourceDisk != "" {
		if projectID := extractProjectFromURL(img.SourceDisk); projectID != "" {
			return projectID
		}
	}

	// If source image exists, try to extract from that
	if img.SourceImage != "" {
		if projectID := extractProjectFromURL(img.SourceImage); projectID != "" {
			return projectID
		}
	}

	return "-"
}

// extractProjectFromURL extracts project ID from GCP resource URL
func extractProjectFromURL(url string) string {
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if part == "projects" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// imageDetailsFromAPI converts full API image to ImageDetails struct
func imageDetailsFromAPI(img *compute.Image) *ImageDetails {
	arch := img.Architecture
	if arch == "" {
		arch = "X86_64" // Default
	}

	details := &ImageDetails{
		Name:                      img.Name,
		ID:                        img.Id,
		Description:               img.Description,
		Family:                    img.Family,
		Status:                    img.Status,
		CreatedAt:                 img.CreationTimestamp,
		CreatedBy:                 extractCreatorFromImage(img),
		Architecture:              arch,
		Labels:                    img.Labels,
		DiskSizeGB:                img.DiskSizeGb,
		ArchiveSizeB:              img.ArchiveSizeBytes,
		SourceType:                img.SourceType,
		SourceDisk:                extractName(img.SourceDisk),
		SourceDiskID:              img.SourceDiskId,
		SourceSnapshot:            extractName(img.SourceSnapshot),
		SourceImage:               extractName(img.SourceImage),
		SourceImageID:             img.SourceImageId,
		EnableConfidentialCompute: img.EnableConfidentialCompute,
		SatisfiesPzs:              img.SatisfiesPzs,
	}

	// Storage locations
	if len(img.StorageLocations) > 0 {
		details.StorageLocations = img.StorageLocations
		details.StorageBytes = img.ArchiveSizeBytes
	}

	// Guest OS features
	for _, feature := range img.GuestOsFeatures {
		details.GuestOSFeatures = append(details.GuestOSFeatures, feature.Type)
	}

	// Licenses
	for _, license := range img.Licenses {
		details.Licenses = append(details.Licenses, extractName(license))
	}

	// License codes
	if len(img.LicenseCodes) > 0 {
		details.LicenseCodes = make([]int64, len(img.LicenseCodes))
		copy(details.LicenseCodes, img.LicenseCodes)
	}

	// Deprecation status
	if img.Deprecated != nil {
		details.Deprecated = &DeprecationStatus{
			State:       img.Deprecated.State,
			Replacement: extractName(img.Deprecated.Replacement),
			Deprecated:  img.Deprecated.Deprecated,
			Obsolete:    img.Deprecated.Obsolete,
			Deleted:     img.Deprecated.Deleted,
		}
	}

	// Determine encryption type
	details.ImageEncryptionKey = "Google-managed"
	if img.ImageEncryptionKey != nil {
		switch {
		case img.ImageEncryptionKey.KmsKeyName != "":
			details.ImageEncryptionKey = "Customer-managed (CMEK)"
		case img.ImageEncryptionKey.RawKey != "":
			details.ImageEncryptionKey = "Customer-supplied (CSEK)"
		}
	}

	// Source disk encryption
	if img.SourceDiskEncryptionKey != nil {
		switch {
		case img.SourceDiskEncryptionKey.KmsKeyName != "":
			details.SourceDiskEncryptionKey = "Customer-managed (CMEK)"
		case img.SourceDiskEncryptionKey.RawKey != "":
			details.SourceDiskEncryptionKey = "Customer-supplied (CSEK)"
		default:
			details.SourceDiskEncryptionKey = "Google-managed"
		}
	}

	// Source snapshot encryption
	if img.SourceSnapshotEncryptionKey != nil {
		switch {
		case img.SourceSnapshotEncryptionKey.KmsKeyName != "":
			details.SourceSnapshotEncryptionKey = "Customer-managed (CMEK)"
		case img.SourceSnapshotEncryptionKey.RawKey != "":
			details.SourceSnapshotEncryptionKey = "Customer-supplied (CSEK)"
		default:
			details.SourceSnapshotEncryptionKey = "Google-managed"
		}
	}

	return details
}

// snapshotFromAPI converts API snapshot to our simplified struct
func snapshotFromAPI(s *compute.Snapshot) Snapshot {
	return Snapshot{
		Name:             s.Name,
		SourceDisk:       extractName(s.SourceDisk),
		SourceDiskID:     s.SourceDiskId,
		SizeGB:           s.DiskSizeGb,
		Status:           s.Status,
		CreatedAt:        s.CreationTimestamp,
		StorageBytes:     s.StorageBytes,
		StorageLocations: s.StorageLocations,
	}
}

// snapshotDetailsFromAPI converts full API snapshot to SnapshotDetails struct
func snapshotDetailsFromAPI(s *compute.Snapshot) *SnapshotDetails {
	details := &SnapshotDetails{
		Name:              s.Name,
		ID:                s.Id,
		Description:       s.Description,
		Status:            s.Status,
		CreatedAt:         s.CreationTimestamp,
		Labels:            s.Labels,
		SourceDisk:        extractName(s.SourceDisk),
		SourceDiskID:      s.SourceDisk,
		DiskSizeGB:        s.DiskSizeGb,
		SizeGB:            s.DiskSizeGb,
		StorageBytes:      s.StorageBytes,
		StorageLocations:  s.StorageLocations,
		DownloadBytes:     s.DownloadBytes,
		AutoCreated:       s.AutoCreated,
		ChainName:         s.ChainName,
		SatisfiesPZS:      s.SatisfiesPzs,
		SnapshotType:      s.SnapshotType,
		CreationSizeBytes: s.CreationSizeBytes,
	}

	// Calculate storage in GB
	if s.StorageBytes > 0 {
		details.StorageBytesGb = s.StorageBytes / (1024 * 1024 * 1024)
	}

	// Extract source disk zone from full URL
	if s.SourceDisk != "" {
		parts := strings.Split(s.SourceDisk, "/")
		for i, part := range parts {
			if part == "zones" && i+1 < len(parts) {
				details.SourceDiskZone = parts[i+1]
				break
			}
		}
	}

	// Determine encryption type
	details.SnapshotEncryptionKey = "Google-managed"
	if s.SnapshotEncryptionKey != nil {
		switch {
		case s.SnapshotEncryptionKey.KmsKeyName != "":
			details.SnapshotEncryptionKey = "Customer-managed (CMEK)"
		case s.SnapshotEncryptionKey.RawKey != "":
			details.SnapshotEncryptionKey = "Customer-supplied (CSEK)"
		}
	}

	details.SourceDiskEncryption = "Google-managed"
	if s.SourceDiskEncryptionKey != nil {
		switch {
		case s.SourceDiskEncryptionKey.KmsKeyName != "":
			details.SourceDiskEncryption = "Customer-managed (CMEK)"
		case s.SourceDiskEncryptionKey.RawKey != "":
			details.SourceDiskEncryption = "Customer-supplied (CSEK)"
		}
	}

	return details
}

// IsReady returns true if the snapshot is in READY state
func (s *Snapshot) IsReady() bool {
	return s.Status == "READY"
}

// IsCreating returns true if the snapshot is being created
func (s *Snapshot) IsCreating() bool {
	return s.Status == "CREATING" || s.Status == "UPLOADING"
}

// IsFailed returns true if the snapshot creation failed
func (s *Snapshot) IsFailed() bool {
	return s.Status == "FAILED"
}

// DeleteDisk deletes a persistent disk. The disk must be detached from all instances.
func (c *ComputeClient) DeleteDisk(ctx context.Context, projectID, zone, diskName string) error {
	_, err := c.service.Disks.Delete(projectID, zone, diskName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "delete disk", diskName)
	}
	return nil
}

// SnapshotCreateConfig holds configuration for creating a snapshot
type SnapshotCreateConfig struct {
	Name            string
	Description     string
	Labels          map[string]string
	StorageLocation string // Regional or multi-regional (e.g., "us-central1", "us", "eu")
}

// CreateSnapshotFromDisk creates a snapshot from a disk with the given configuration
func (c *ComputeClient) CreateSnapshotFromDisk(ctx context.Context, projectID, zone, diskName string, config SnapshotCreateConfig) error {
	snapshot := &compute.Snapshot{
		Name:        config.Name,
		Description: config.Description,
	}

	if len(config.Labels) > 0 {
		snapshot.Labels = config.Labels
	}

	if config.StorageLocation != "" {
		snapshot.StorageLocations = []string{config.StorageLocation}
	}

	_, err := c.service.Disks.CreateSnapshot(projectID, zone, diskName, snapshot).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "create snapshot from disk", diskName)
	}
	return nil
}

// ImageCreateConfig holds configuration for creating an image
type ImageCreateConfig struct {
	Name            string
	Description     string
	Family          string
	Labels          map[string]string
	StorageLocation string // Regional or multi-regional (e.g., "us-central1", "us", "eu")
	ForceCreate     bool   // If true, create even if disk is attached to running instance
}

// CreateImageFromDisk creates an image from a disk with the given configuration
func (c *ComputeClient) CreateImageFromDisk(ctx context.Context, projectID, zone, diskName string, config ImageCreateConfig) error {
	// Build the full source disk URL
	sourceDisk := fmt.Sprintf("projects/%s/zones/%s/disks/%s", projectID, zone, diskName)

	image := &compute.Image{
		Name:        config.Name,
		Description: config.Description,
		SourceDisk:  sourceDisk,
	}

	if config.Family != "" {
		image.Family = config.Family
	}

	if len(config.Labels) > 0 {
		image.Labels = config.Labels
	}

	if config.StorageLocation != "" {
		image.StorageLocations = []string{config.StorageLocation}
	}

	call := c.service.Images.Insert(projectID, image).Context(ctx)
	if config.ForceCreate {
		call = call.ForceCreate(true)
	}

	_, err := call.Do()
	if err != nil {
		return WrapActionError(err, "create image from disk", diskName)
	}
	return nil
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
