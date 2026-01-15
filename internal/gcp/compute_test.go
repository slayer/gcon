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

func TestDiskFromAPI(t *testing.T) {
	tests := []struct {
		name     string
		disk     *compute.Disk
		zone     string
		validate func(t *testing.T, disk Disk)
	}{
		{
			name: "basic disk",
			disk: &compute.Disk{
				Name:              "test-disk",
				SizeGb:            100,
				Type:              "projects/test/zones/us-central1-a/diskTypes/pd-standard",
				Status:            "READY",
				CreationTimestamp: "2025-01-11T10:00:00Z",
			},
			zone: "zones/us-central1-a",
			validate: func(t *testing.T, d Disk) {
				assert.Equal(t, "test-disk", d.Name)
				assert.Equal(t, "us-central1-a", d.Zone)
				assert.Equal(t, int64(100), d.SizeGB)
				assert.Equal(t, "pd-standard", d.Type)
				assert.Equal(t, "READY", d.Status)
				assert.Empty(t, d.AttachedTo)
			},
		},
		{
			name: "disk attached to instance",
			disk: &compute.Disk{
				Name:   "attached-disk",
				SizeGb: 500,
				Type:   "projects/test/zones/us-central1-a/diskTypes/pd-ssd",
				Status: "READY",
				Users: []string{
					"projects/test/zones/us-central1-a/instances/my-vm",
				},
			},
			zone: "zones/us-central1-a",
			validate: func(t *testing.T, d Disk) {
				assert.Equal(t, "attached-disk", d.Name)
				assert.Equal(t, "pd-ssd", d.Type)
				assert.Equal(t, "my-vm", d.AttachedTo)
			},
		},
		{
			name: "disk with balanced type",
			disk: &compute.Disk{
				Name:   "balanced-disk",
				SizeGb: 200,
				Type:   "projects/test/zones/us-east1-b/diskTypes/pd-balanced",
				Status: "READY",
			},
			zone: "us-east1-b",
			validate: func(t *testing.T, d Disk) {
				assert.Equal(t, "us-east1-b", d.Zone)
				assert.Equal(t, "pd-balanced", d.Type)
				assert.Equal(t, int64(200), d.SizeGB)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disk := diskFromAPI(tt.disk, tt.zone)
			tt.validate(t, disk)
		})
	}
}

func TestDiskMethods(t *testing.T) {
	t.Run("IsAttached", func(t *testing.T) {
		attached := Disk{AttachedTo: "my-instance"}
		detached := Disk{AttachedTo: ""}

		assert.True(t, attached.IsAttached())
		assert.False(t, detached.IsAttached())
	})

	t.Run("IsReady", func(t *testing.T) {
		ready := Disk{Status: "READY"}
		creating := Disk{Status: "CREATING"}
		failed := Disk{Status: "FAILED"}

		assert.True(t, ready.IsReady())
		assert.False(t, creating.IsReady())
		assert.False(t, failed.IsReady())
	})
}

func TestDiskDetailsFromAPI(t *testing.T) {
	tests := []struct {
		name     string
		disk     *compute.Disk
		zone     string
		validate func(t *testing.T, details *DiskDetails)
	}{
		{
			name: "basic disk",
			disk: &compute.Disk{
				Name:              "test-disk",
				Id:                12345,
				Description:       "Test disk",
				Status:            "READY",
				SizeGb:            100,
				Type:              "projects/test/zones/us-central1-a/diskTypes/pd-ssd",
				CreationTimestamp: "2025-01-11T10:00:00Z",
			},
			zone: "zones/us-central1-a",
			validate: func(t *testing.T, d *DiskDetails) {
				assert.Equal(t, "test-disk", d.Name)
				assert.Equal(t, uint64(12345), d.ID)
				assert.Equal(t, "Test disk", d.Description)
				assert.Equal(t, "READY", d.Status)
				assert.Equal(t, "us-central1-a", d.Zone)
				assert.Equal(t, int64(100), d.SizeGB)
				assert.Equal(t, "pd-ssd", d.Type)
				assert.Equal(t, "Google-managed", d.DiskEncryptionKey)
			},
		},
		{
			name: "disk with users",
			disk: &compute.Disk{
				Name:   "attached-disk",
				Id:     67890,
				Status: "READY",
				SizeGb: 500,
				Type:   "projects/test/zones/us-central1-a/diskTypes/pd-balanced",
				Users: []string{
					"projects/test/zones/us-central1-a/instances/vm-1",
					"projects/test/zones/us-central1-a/instances/vm-2",
				},
			},
			zone: "us-central1-a",
			validate: func(t *testing.T, d *DiskDetails) {
				assert.Equal(t, "attached-disk", d.Name)
				assert.Len(t, d.Users, 2)
				assert.Equal(t, "vm-1", d.Users[0])
				assert.Equal(t, "vm-2", d.Users[1])
			},
		},
		{
			name: "disk with source image",
			disk: &compute.Disk{
				Name:        "boot-disk",
				Id:          11111,
				Status:      "READY",
				SizeGb:      50,
				Type:        "pd-standard",
				SourceImage: "projects/debian-cloud/global/images/debian-11-bullseye-v20230711",
			},
			zone: "us-east1-b",
			validate: func(t *testing.T, d *DiskDetails) {
				assert.Equal(t, "debian-11-bullseye-v20230711", d.SourceImage)
				assert.Empty(t, d.SourceSnapshot)
				assert.Empty(t, d.SourceDisk)
			},
		},
		{
			name: "disk with CMEK encryption",
			disk: &compute.Disk{
				Name:   "encrypted-disk",
				Id:     22222,
				Status: "READY",
				SizeGb: 200,
				Type:   "pd-ssd",
				DiskEncryptionKey: &compute.CustomerEncryptionKey{
					KmsKeyName: "projects/test/locations/global/keyRings/my-ring/cryptoKeys/my-key",
				},
			},
			zone: "us-west1-a",
			validate: func(t *testing.T, d *DiskDetails) {
				assert.Equal(t, "Customer-managed (CMEK)", d.DiskEncryptionKey)
			},
		},
		{
			name: "disk with provisioned IOPS",
			disk: &compute.Disk{
				Name:                  "extreme-disk",
				Id:                    33333,
				Status:                "READY",
				SizeGb:                1000,
				Type:                  "pd-extreme",
				ProvisionedIops:       50000,
				ProvisionedThroughput: 1200,
			},
			zone: "us-central1-a",
			validate: func(t *testing.T, d *DiskDetails) {
				assert.Equal(t, int64(50000), d.ProvisionedIOPS)
				assert.Equal(t, int64(1200), d.ProvisionedTPUT)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details := diskDetailsFromAPI(tt.disk, tt.zone)
			tt.validate(t, details)
		})
	}
}

func TestSnapshotFromAPI(t *testing.T) {
	tests := []struct {
		name     string
		snapshot *compute.Snapshot
		validate func(t *testing.T, s Snapshot)
	}{
		{
			name: "basic snapshot",
			snapshot: &compute.Snapshot{
				Name:              "snapshot-1",
				SourceDisk:        "projects/test/zones/us-central1-a/disks/my-disk",
				SourceDiskId:      "1234567890",
				DiskSizeGb:        100,
				Status:            "READY",
				CreationTimestamp: "2025-01-11T10:00:00Z",
				StorageBytes:      50000000000,
			},
			validate: func(t *testing.T, s Snapshot) {
				assert.Equal(t, "snapshot-1", s.Name)
				assert.Equal(t, "my-disk", s.SourceDisk)
				assert.Equal(t, "1234567890", s.SourceDiskID)
				assert.Equal(t, int64(100), s.SizeGB)
				assert.Equal(t, "READY", s.Status)
				assert.Equal(t, int64(50000000000), s.StorageBytes)
			},
		},
		{
			name: "snapshot with full disk path",
			snapshot: &compute.Snapshot{
				Name:       "snapshot-2",
				SourceDisk: "https://www.googleapis.com/compute/v1/projects/test/zones/us-east1-b/disks/data-disk",
				DiskSizeGb: 500,
				Status:     "CREATING",
			},
			validate: func(t *testing.T, s Snapshot) {
				assert.Equal(t, "data-disk", s.SourceDisk)
				assert.Equal(t, int64(500), s.SizeGB)
				assert.Equal(t, "CREATING", s.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := snapshotFromAPI(tt.snapshot)
			tt.validate(t, snapshot)
		})
	}
}

func TestSnapshotMethods(t *testing.T) {
	t.Run("IsReady", func(t *testing.T) {
		ready := Snapshot{Status: "READY"}
		creating := Snapshot{Status: "CREATING"}
		failed := Snapshot{Status: "FAILED"}

		assert.True(t, ready.IsReady())
		assert.False(t, creating.IsReady())
		assert.False(t, failed.IsReady())
	})

	t.Run("IsCreating", func(t *testing.T) {
		creating := Snapshot{Status: "CREATING"}
		uploading := Snapshot{Status: "UPLOADING"}
		ready := Snapshot{Status: "READY"}

		assert.True(t, creating.IsCreating())
		assert.True(t, uploading.IsCreating())
		assert.False(t, ready.IsCreating())
	})

	t.Run("IsFailed", func(t *testing.T) {
		failed := Snapshot{Status: "FAILED"}
		ready := Snapshot{Status: "READY"}
		creating := Snapshot{Status: "CREATING"}

		assert.True(t, failed.IsFailed())
		assert.False(t, ready.IsFailed())
		assert.False(t, creating.IsFailed())
	})
}

func TestSnapshotDetailsFromAPI(t *testing.T) {
	tests := []struct {
		name     string
		snapshot *compute.Snapshot
		validate func(t *testing.T, details *SnapshotDetails)
	}{
		{
			name: "basic snapshot",
			snapshot: &compute.Snapshot{
				Name:              "snapshot-1",
				Id:                12345,
				Description:       "Test snapshot",
				Status:            "READY",
				CreationTimestamp: "2025-01-11T10:00:00Z",
				SourceDisk:        "projects/test/zones/us-central1-a/disks/my-disk",
				DiskSizeGb:        100,
				StorageBytes:      50000000000,
				Labels:            map[string]string{"env": "prod"},
			},
			validate: func(t *testing.T, d *SnapshotDetails) {
				assert.Equal(t, "snapshot-1", d.Name)
				assert.Equal(t, uint64(12345), d.ID)
				assert.Equal(t, "Test snapshot", d.Description)
				assert.Equal(t, "READY", d.Status)
				assert.Equal(t, "my-disk", d.SourceDisk)
				assert.Equal(t, "us-central1-a", d.SourceDiskZone)
				assert.Equal(t, int64(100), d.SizeGB)
				assert.Equal(t, int64(50000000000), d.StorageBytes)
				assert.Equal(t, int64(46), d.StorageBytesGb) // 50000000000 / 1024^3
				assert.Equal(t, "Google-managed", d.SnapshotEncryptionKey)
			},
		},
		{
			name: "snapshot with CMEK encryption",
			snapshot: &compute.Snapshot{
				Name:       "encrypted-snapshot",
				Id:         67890,
				Status:     "READY",
				SourceDisk: "projects/test/zones/us-west1-a/disks/secure-disk",
				DiskSizeGb: 200,
				SnapshotEncryptionKey: &compute.CustomerEncryptionKey{
					KmsKeyName: "projects/test/locations/global/keyRings/ring/cryptoKeys/key",
				},
				SourceDiskEncryptionKey: &compute.CustomerEncryptionKey{
					KmsKeyName: "projects/test/locations/global/keyRings/ring/cryptoKeys/key",
				},
			},
			validate: func(t *testing.T, d *SnapshotDetails) {
				assert.Equal(t, "encrypted-snapshot", d.Name)
				assert.Equal(t, "us-west1-a", d.SourceDiskZone)
				assert.Equal(t, "Customer-managed (CMEK)", d.SnapshotEncryptionKey)
				assert.Equal(t, "Customer-managed (CMEK)", d.SourceDiskEncryption)
			},
		},
		{
			name: "snapshot with storage locations",
			snapshot: &compute.Snapshot{
				Name:             "multi-region-snapshot",
				Id:               11111,
				Status:           "READY",
				SourceDisk:       "projects/test/zones/europe-west1-b/disks/data",
				DiskSizeGb:       500,
				StorageLocations: []string{"eu", "us"},
				SnapshotType:     "STANDARD",
				AutoCreated:      false,
			},
			validate: func(t *testing.T, d *SnapshotDetails) {
				assert.Equal(t, "multi-region-snapshot", d.Name)
				assert.Equal(t, "europe-west1-b", d.SourceDiskZone)
				assert.Len(t, d.StorageLocations, 2)
				assert.Equal(t, "eu", d.StorageLocations[0])
				assert.Equal(t, "us", d.StorageLocations[1])
				assert.Equal(t, "STANDARD", d.SnapshotType)
				assert.False(t, d.AutoCreated)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details := snapshotDetailsFromAPI(tt.snapshot)
			tt.validate(t, details)
		})
	}
}
