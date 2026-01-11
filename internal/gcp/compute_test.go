package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/compute/v1"
)

func TestExtractName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full zone path",
			input:    "zones/us-central1-a",
			expected: "us-central1-a",
		},
		{
			name:     "full machine type path",
			input:    "projects/my-project/zones/us-central1-a/machineTypes/n1-standard-1",
			expected: "n1-standard-1",
		},
		{
			name:     "simple name",
			input:    "my-instance",
			expected: "my-instance",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "network path",
			input:    "projects/my-project/global/networks/default",
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInstanceDetailsFromAPI(t *testing.T) {
	// Helper to create a bool pointer
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name     string
		inst     *compute.Instance
		zone     string
		validate func(t *testing.T, details *InstanceDetails)
	}{
		{
			name: "basic instance",
			inst: &compute.Instance{
				Name:               "test-instance",
				Id:                 12345,
				Description:        "Test description",
				Status:             "RUNNING",
				CreationTimestamp:  "2025-01-11T10:00:00Z",
				DeletionProtection: true,
				MachineType:        "projects/test/zones/us-central1-a/machineTypes/n1-standard-1",
				CpuPlatform:        "Intel Haswell",
				Labels:             map[string]string{"env": "test"},
				Tags:               &compute.Tags{Items: []string{"http-server", "https-server"}},
			},
			zone: "zones/us-central1-a",
			validate: func(t *testing.T, d *InstanceDetails) {
				assert.Equal(t, "test-instance", d.Name)
				assert.Equal(t, uint64(12345), d.ID)
				assert.Equal(t, "Test description", d.Description)
				assert.Equal(t, "RUNNING", d.Status)
				assert.Equal(t, "us-central1-a", d.Zone)
				assert.Equal(t, true, d.DeletionProtection)
				assert.Equal(t, "n1-standard-1", d.MachineType)
				assert.Equal(t, "Intel Haswell", d.CpuPlatform)
				assert.Equal(t, map[string]string{"env": "test"}, d.Labels)
				assert.Equal(t, []string{"http-server", "https-server"}, d.Tags)
			},
		},
		{
			name: "instance with network interfaces",
			inst: &compute.Instance{
				Name:        "network-test",
				Id:          67890,
				Status:      "RUNNING",
				MachineType: "n2-standard-2",
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						Name:       "nic0",
						Network:    "projects/test/global/networks/default",
						Subnetwork: "projects/test/regions/us-central1/subnetworks/default",
						NetworkIP:  "10.128.0.2",
						NicType:    "GVNIC",
						StackType:  "IPV4_ONLY",
						AccessConfigs: []*compute.AccessConfig{
							{
								NatIP:       "35.192.0.1",
								NetworkTier: "PREMIUM",
							},
						},
					},
				},
			},
			zone: "us-central1-a",
			validate: func(t *testing.T, d *InstanceDetails) {
				assert.Len(t, d.NetworkInterfaces, 1)
				nic := d.NetworkInterfaces[0]
				assert.Equal(t, "nic0", nic.Name)
				assert.Equal(t, "default", nic.Network)
				assert.Equal(t, "default", nic.Subnetwork)
				assert.Equal(t, "10.128.0.2", nic.InternalIP)
				assert.Equal(t, "35.192.0.1", nic.ExternalIP)
				assert.Equal(t, "GVNIC", nic.NicType)
				assert.Equal(t, "PREMIUM", nic.Tier)
			},
		},
		{
			name: "instance with disks",
			inst: &compute.Instance{
				Name:        "disk-test",
				Id:          11111,
				Status:      "RUNNING",
				MachineType: "e2-medium",
				Disks: []*compute.AttachedDisk{
					{
						Boot:       true,
						AutoDelete: true,
						Mode:       "READ_WRITE",
						DiskSizeGb: 100,
						DeviceName: "boot-disk",
						Source:     "projects/test/zones/us-central1-a/disks/test-boot-disk",
						Interface:  "SCSI",
					},
					{
						Boot:       false,
						AutoDelete: false,
						Mode:       "READ_WRITE",
						DiskSizeGb: 500,
						DeviceName: "data-disk",
						Source:     "projects/test/zones/us-central1-a/disks/test-data-disk",
						Interface:  "NVME",
					},
				},
			},
			zone: "us-central1-a",
			validate: func(t *testing.T, d *InstanceDetails) {
				assert.Len(t, d.Disks, 2)

				bootDisk := d.Disks[0]
				assert.Equal(t, "test-boot-disk", bootDisk.Name)
				assert.True(t, bootDisk.Boot)
				assert.True(t, bootDisk.AutoDelete)
				assert.Equal(t, int64(100), bootDisk.SizeGB)
				assert.Equal(t, "READ_WRITE", bootDisk.Mode)
				assert.Equal(t, "SCSI", bootDisk.Type)

				dataDisk := d.Disks[1]
				assert.Equal(t, "test-data-disk", dataDisk.Name)
				assert.False(t, dataDisk.Boot)
				assert.False(t, dataDisk.AutoDelete)
				assert.Equal(t, int64(500), dataDisk.SizeGB)
			},
		},
		{
			name: "instance with shielded vm config",
			inst: &compute.Instance{
				Name:        "shielded-test",
				Id:          22222,
				Status:      "RUNNING",
				MachineType: "n1-standard-1",
				ShieldedInstanceConfig: &compute.ShieldedInstanceConfig{
					EnableSecureBoot:          true,
					EnableVtpm:                true,
					EnableIntegrityMonitoring: true,
				},
			},
			zone: "us-central1-a",
			validate: func(t *testing.T, d *InstanceDetails) {
				assert.True(t, d.ShieldedVM.SecureBoot)
				assert.True(t, d.ShieldedVM.VTPM)
				assert.True(t, d.ShieldedVM.IntegrityMonitoring)
			},
		},
		{
			name: "instance with scheduling",
			inst: &compute.Instance{
				Name:        "scheduling-test",
				Id:          33333,
				Status:      "RUNNING",
				MachineType: "n1-standard-1",
				Scheduling: &compute.Scheduling{
					ProvisioningModel:         "STANDARD",
					Preemptible:               false,
					OnHostMaintenance:         "MIGRATE",
					AutomaticRestart:          boolPtr(true),
					InstanceTerminationAction: "STOP",
				},
			},
			zone: "us-central1-a",
			validate: func(t *testing.T, d *InstanceDetails) {
				assert.Equal(t, "STANDARD", d.Scheduling.ProvisioningModel)
				assert.False(t, d.Scheduling.Preemptible)
				assert.Equal(t, "MIGRATE", d.Scheduling.OnHostMaintenance)
				assert.True(t, d.Scheduling.AutomaticRestart)
				assert.Equal(t, "STOP", d.Scheduling.InstanceTerminationAction)
			},
		},
		{
			name: "instance with service account",
			inst: &compute.Instance{
				Name:        "sa-test",
				Id:          44444,
				Status:      "RUNNING",
				MachineType: "n1-standard-1",
				ServiceAccounts: []*compute.ServiceAccount{
					{
						Email: "123456-compute@developer.gserviceaccount.com",
						Scopes: []string{
							"https://www.googleapis.com/auth/compute",
							"https://www.googleapis.com/auth/devstorage.read_only",
						},
					},
				},
			},
			zone: "us-central1-a",
			validate: func(t *testing.T, d *InstanceDetails) {
				assert.Equal(t, "123456-compute@developer.gserviceaccount.com", d.ServiceAccount)
				assert.Len(t, d.Scopes, 2)
			},
		},
		{
			name: "instance with GPUs",
			inst: &compute.Instance{
				Name:        "gpu-test",
				Id:          55555,
				Status:      "RUNNING",
				MachineType: "n1-standard-8",
				GuestAccelerators: []*compute.AcceleratorConfig{
					{
						AcceleratorType:  "projects/test/zones/us-central1-a/acceleratorTypes/nvidia-tesla-t4",
						AcceleratorCount: 2,
					},
				},
			},
			zone: "us-central1-a",
			validate: func(t *testing.T, d *InstanceDetails) {
				assert.Len(t, d.GPUs, 1)
				assert.Equal(t, "nvidia-tesla-t4", d.GPUs[0].Type)
				assert.Equal(t, int64(2), d.GPUs[0].Count)
			},
		},
		{
			name: "instance with metadata",
			inst: &compute.Instance{
				Name:        "metadata-test",
				Id:          66666,
				Status:      "RUNNING",
				MachineType: "n1-standard-1",
				Metadata: &compute.Metadata{
					Items: []*compute.MetadataItems{
						{Key: "startup-script", Value: stringPtr("#!/bin/bash\necho hello")},
						{Key: "enable-oslogin", Value: stringPtr("TRUE")},
					},
				},
			},
			zone: "us-central1-a",
			validate: func(t *testing.T, d *InstanceDetails) {
				assert.Len(t, d.Metadata, 2)
				assert.Equal(t, "#!/bin/bash\necho hello", d.Metadata["startup-script"])
				assert.Equal(t, "TRUE", d.Metadata["enable-oslogin"])
			},
		},
		{
			name: "minimal instance with nil fields",
			inst: &compute.Instance{
				Name:        "minimal-test",
				Id:          77777,
				Status:      "STOPPED",
				MachineType: "e2-micro",
			},
			zone: "us-west1-b",
			validate: func(t *testing.T, d *InstanceDetails) {
				assert.Equal(t, "minimal-test", d.Name)
				assert.Equal(t, uint64(77777), d.ID)
				assert.Equal(t, "STOPPED", d.Status)
				assert.Equal(t, "us-west1-b", d.Zone)
				assert.Equal(t, "e2-micro", d.MachineType)
				assert.Empty(t, d.Labels)
				assert.Empty(t, d.Tags)
				assert.Empty(t, d.NetworkInterfaces)
				assert.Empty(t, d.Disks)
				assert.Empty(t, d.GPUs)
				assert.Empty(t, d.Metadata)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details := instanceDetailsFromAPI(tt.inst, tt.zone)
			tt.validate(t, details)
		})
	}
}

func TestInstanceFromAPI(t *testing.T) {
	tests := []struct {
		name     string
		inst     *compute.Instance
		zone     string
		validate func(t *testing.T, instance Instance)
	}{
		{
			name: "basic instance",
			inst: &compute.Instance{
				Name:              "test-vm",
				Status:            "RUNNING",
				CreationTimestamp: "2025-01-11T10:00:00Z",
				MachineType:       "projects/test/zones/us-central1-a/machineTypes/n1-standard-4",
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						NetworkIP: "10.0.0.5",
						AccessConfigs: []*compute.AccessConfig{
							{NatIP: "35.200.0.1"},
						},
					},
				},
			},
			zone: "zones/us-central1-a",
			validate: func(t *testing.T, i Instance) {
				assert.Equal(t, "test-vm", i.Name)
				assert.Equal(t, "us-central1-a", i.Zone)
				assert.Equal(t, "RUNNING", i.Status)
				assert.Equal(t, "n1-standard-4", i.MachineType)
				assert.Equal(t, "10.0.0.5", i.InternalIP)
				assert.Equal(t, "35.200.0.1", i.ExternalIP)
			},
		},
		{
			name: "instance without external IP",
			inst: &compute.Instance{
				Name:        "internal-only",
				Status:      "RUNNING",
				MachineType: "e2-medium",
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						NetworkIP: "10.128.0.10",
					},
				},
			},
			zone: "us-east1-b",
			validate: func(t *testing.T, i Instance) {
				assert.Equal(t, "10.128.0.10", i.InternalIP)
				assert.Empty(t, i.ExternalIP)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := instanceFromAPI(tt.inst, tt.zone)
			tt.validate(t, instance)
		})
	}
}

func TestInstanceMethods(t *testing.T) {
	t.Run("IsRunning", func(t *testing.T) {
		running := Instance{Status: "RUNNING"}
		stopped := Instance{Status: "STOPPED"}

		assert.True(t, running.IsRunning())
		assert.False(t, stopped.IsRunning())
	})

	t.Run("IsStopped", func(t *testing.T) {
		stopped := Instance{Status: "STOPPED"}
		terminated := Instance{Status: "TERMINATED"}
		running := Instance{Status: "RUNNING"}

		assert.True(t, stopped.IsStopped())
		assert.True(t, terminated.IsStopped())
		assert.False(t, running.IsStopped())
	})

	t.Run("ZoneFromInstance", func(t *testing.T) {
		inst := Instance{Zone: "us-central1-a"}
		assert.Equal(t, "us-central1-a", inst.ZoneFromInstance())
	})
}

func stringPtr(s string) *string {
	return &s
}
