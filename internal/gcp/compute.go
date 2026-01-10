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
