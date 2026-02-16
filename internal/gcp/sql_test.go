package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

func TestFormatDatabaseVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "MySQL 8.0",
			input:    "MYSQL_8_0",
			expected: "MySQL 8.0",
		},
		{
			name:     "MySQL 5.7",
			input:    "MYSQL_5_7",
			expected: "MySQL 5.7",
		},
		{
			name:     "PostgreSQL 15",
			input:    "POSTGRES_15",
			expected: "PostgreSQL 15",
		},
		{
			name:     "PostgreSQL 14",
			input:    "POSTGRES_14",
			expected: "PostgreSQL 14",
		},
		{
			name:     "SQL Server 2019 Standard",
			input:    "SQLSERVER_2019_STANDARD",
			expected: "SQL Server 2019",
		},
		{
			name:     "SQL Server 2022 Enterprise",
			input:    "SQLSERVER_2022_ENTERPRISE",
			expected: "SQL Server 2022",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "unknown version returns as-is",
			input:    "UNKNOWN_VERSION",
			expected: "UNKNOWN_VERSION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDatabaseVersion(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSQLInstanceFromAPI(t *testing.T) {
	inst := &sqladmin.DatabaseInstance{
		Name:            "my-db-instance",
		DatabaseVersion: "POSTGRES_15",
		State:           "RUNNABLE",
		Region:          "us-central1",
		Settings: &sqladmin.Settings{
			Tier: "db-custom-4-16384",
		},
		IpAddresses: []*sqladmin.IpMapping{
			{IpAddress: "10.0.0.1", Type: "PRIVATE"},
			{IpAddress: "34.1.2.3", Type: "PRIMARY"},
		},
		CreateTime: "2024-06-15T10:30:00.000Z",
	}

	result := sqlInstanceFromAPI(inst)

	assert.Equal(t, "my-db-instance", result.Name)
	assert.Equal(t, "POSTGRES_15", result.DatabaseVersion)
	assert.Equal(t, "RUNNABLE", result.State)
	assert.Equal(t, "us-central1", result.Region)
	assert.Equal(t, "db-custom-4-16384", result.Tier)
	assert.Equal(t, "34.1.2.3", result.PrimaryIP)
	assert.Equal(t, "2024-06-15T10:30:00.000Z", result.CreatedAt)
}

func TestSQLInstanceDetailsFromAPI(t *testing.T) {
	inst := &sqladmin.DatabaseInstance{
		Name:            "prod-postgres",
		DatabaseVersion: "POSTGRES_15",
		State:           "RUNNABLE",
		Region:          "us-central1",
		ConnectionName:  "my-project:us-central1:prod-postgres",
		IpAddresses: []*sqladmin.IpMapping{
			{IpAddress: "34.1.2.3", Type: "PRIMARY"},
			{IpAddress: "10.0.0.5", Type: "PRIVATE"},
		},
		Settings: &sqladmin.Settings{
			Tier:             "db-custom-4-16384",
			DataDiskSizeGb:   100,
			DataDiskType:     "PD_SSD",
			AvailabilityType: "REGIONAL",
			StorageAutoResize: boolPtr(true),
			BackupConfiguration: &sqladmin.BackupConfiguration{
				Enabled:                    true,
				BinaryLogEnabled:           true,
				PointInTimeRecoveryEnabled: true,
			},
			MaintenanceWindow: &sqladmin.MaintenanceWindow{
				Day:  7, // Sunday
				Hour: 4,
			},
			DatabaseFlags: []*sqladmin.DatabaseFlags{
				{Name: "max_connections", Value: "200"},
				{Name: "log_min_duration_statement", Value: "1000"},
			},
		},
		ReplicaNames:       []string{"read-replica-1", "read-replica-2"},
		MasterInstanceName: "primary-instance",
		CreateTime:         "2024-03-10T08:00:00.000Z",
	}

	details := sqlInstanceDetailsFromAPI(inst)

	// Basic fields
	assert.Equal(t, "prod-postgres", details.Name)
	assert.Equal(t, "POSTGRES_15", details.DatabaseVersion)
	assert.Equal(t, "RUNNABLE", details.State)
	assert.Equal(t, "us-central1", details.Region)
	assert.Equal(t, "my-project:us-central1:prod-postgres", details.ConnectionName)
	assert.Equal(t, "2024-03-10T08:00:00.000Z", details.CreatedAt)

	// Settings
	assert.Equal(t, "db-custom-4-16384", details.Tier)
	assert.Equal(t, int64(100), details.DiskSizeGB)
	assert.Equal(t, "PD_SSD", details.DiskType)
	assert.Equal(t, "REGIONAL", details.AvailabilityType)
	assert.True(t, details.StorageAutoResize)

	// Backup configuration
	assert.True(t, details.BackupEnabled)
	assert.True(t, details.BinaryLogEnabled)
	assert.True(t, details.PITREnabled)

	// Maintenance window: Day=0 (Sunday), Hour=4
	assert.Equal(t, "Sunday 04:00", details.MaintenanceWindow)

	// Database flags
	assert.Equal(t, map[string]string{
		"max_connections":            "200",
		"log_min_duration_statement": "1000",
	}, details.DatabaseFlags)

	// IP addresses
	assert.Len(t, details.IPs, 2)
	assert.Equal(t, "34.1.2.3", details.IPs[0].IPAddress)
	assert.Equal(t, "PRIMARY", details.IPs[0].Type)
	assert.Equal(t, "10.0.0.5", details.IPs[1].IPAddress)
	assert.Equal(t, "PRIVATE", details.IPs[1].Type)

	// Replicas
	assert.Equal(t, []string{"read-replica-1", "read-replica-2"}, details.ReplicaNames)
	assert.Equal(t, "primary-instance", details.MasterInstanceName)
}

func TestSQLInstanceDetailsFromAPI_NilSettings(t *testing.T) {
	inst := &sqladmin.DatabaseInstance{
		Name:            "bare-instance",
		DatabaseVersion: "MYSQL_8_0",
		State:           "RUNNABLE",
		Region:          "us-east1",
		Settings:        nil,
		CreateTime:      "2024-01-01T00:00:00.000Z",
	}

	details := sqlInstanceDetailsFromAPI(inst)

	assert.Equal(t, "bare-instance", details.Name)
	assert.Equal(t, "MYSQL_8_0", details.DatabaseVersion)
	assert.Equal(t, "RUNNABLE", details.State)
	assert.Equal(t, "us-east1", details.Region)

	// Settings-derived fields should have zero values
	assert.Equal(t, "", details.Tier)
	assert.Equal(t, int64(0), details.DiskSizeGB)
	assert.Equal(t, "", details.DiskType)
	assert.Equal(t, "", details.AvailabilityType)
	assert.False(t, details.StorageAutoResize)
	assert.False(t, details.BackupEnabled)
	assert.False(t, details.BinaryLogEnabled)
	assert.False(t, details.PITREnabled)
	assert.Equal(t, "", details.MaintenanceWindow)
	assert.Empty(t, details.DatabaseFlags)
}

func TestSQLDatabaseFromAPI(t *testing.T) {
	db := &sqladmin.Database{
		Name:      "myapp_production",
		Instance:  "prod-postgres",
		Charset:   "UTF8",
		Collation: "en_US.UTF8",
	}

	result := sqlDatabaseFromAPI(db)

	assert.Equal(t, "myapp_production", result.Name)
	assert.Equal(t, "UTF8", result.Charset)
	assert.Equal(t, "en_US.UTF8", result.Collation)
}

func TestSQLBackupRunFromAPI(t *testing.T) {
	backup := &sqladmin.BackupRun{
		Id:          12345,
		Status:      "SUCCESSFUL",
		Type:        "AUTOMATED",
		StartTime:   "2024-06-15T02:00:00.000Z",
		EndTime:     "2024-06-15T02:05:30.000Z",
		Description: "Daily automated backup",
	}

	result := sqlBackupRunFromAPI(backup)

	assert.Equal(t, int64(12345), result.ID)
	assert.Equal(t, "SUCCESSFUL", result.Status)
	assert.Equal(t, "AUTOMATED", result.Type)
	assert.Equal(t, "2024-06-15T02:00:00.000Z", result.StartTime)
	assert.Equal(t, "2024-06-15T02:05:30.000Z", result.EndTime)
	assert.Equal(t, "Daily automated backup", result.Description)
}

func TestEffectiveState(t *testing.T) {
	tests := []struct {
		name     string
		inst     *sqladmin.DatabaseInstance
		expected string
	}{
		{
			name: "RUNNABLE with ALWAYS stays RUNNABLE",
			inst: &sqladmin.DatabaseInstance{
				State:    "RUNNABLE",
				Settings: &sqladmin.Settings{ActivationPolicy: "ALWAYS"},
			},
			expected: "RUNNABLE",
		},
		{
			name: "RUNNABLE with NEVER becomes STOPPED",
			inst: &sqladmin.DatabaseInstance{
				State:    "RUNNABLE",
				Settings: &sqladmin.Settings{ActivationPolicy: "NEVER"},
			},
			expected: "STOPPED",
		},
		{
			name: "STOPPED with NEVER stays STOPPED",
			inst: &sqladmin.DatabaseInstance{
				State:    "STOPPED",
				Settings: &sqladmin.Settings{ActivationPolicy: "NEVER"},
			},
			expected: "STOPPED",
		},
		{
			name: "STOPPED with ALWAYS stays STOPPED",
			inst: &sqladmin.DatabaseInstance{
				State:    "STOPPED",
				Settings: &sqladmin.Settings{ActivationPolicy: "ALWAYS"},
			},
			expected: "STOPPED",
		},
		{
			name: "nil settings preserves raw state",
			inst: &sqladmin.DatabaseInstance{
				State:    "RUNNABLE",
				Settings: nil,
			},
			expected: "RUNNABLE",
		},
		{
			name: "PENDING_CREATE passes through",
			inst: &sqladmin.DatabaseInstance{
				State:    "PENDING_CREATE",
				Settings: &sqladmin.Settings{ActivationPolicy: "ALWAYS"},
			},
			expected: "PENDING_CREATE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, effectiveState(tt.inst))
		})
	}
}

func TestSQLInstanceFromAPI_EffectiveState(t *testing.T) {
	// Verifies that instances with activationPolicy=NEVER show as STOPPED
	inst := &sqladmin.DatabaseInstance{
		Name:            "stopped-instance",
		DatabaseVersion: "POSTGRES_15",
		State:           "RUNNABLE",
		Region:          "us-central1",
		Settings: &sqladmin.Settings{
			Tier:             "db-f1-micro",
			ActivationPolicy: "NEVER",
		},
		CreateTime: "2024-06-15T10:30:00.000Z",
	}

	result := sqlInstanceFromAPI(inst)
	assert.Equal(t, "STOPPED", result.State)
	assert.True(t, result.IsStopped())
	assert.False(t, result.IsRunnable())
}

func TestIsRunnable(t *testing.T) {
	tests := []struct {
		name     string
		state    string
		expected bool
	}{
		{
			name:     "RUNNABLE is runnable",
			state:    "RUNNABLE",
			expected: true,
		},
		{
			name:     "STOPPED is not runnable",
			state:    "STOPPED",
			expected: false,
		},
		{
			name:     "SUSPENDED is not runnable",
			state:    "SUSPENDED",
			expected: false,
		},
		{
			name:     "PENDING_CREATE is not runnable",
			state:    "PENDING_CREATE",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := &SQLInstance{State: tt.state}
			assert.Equal(t, tt.expected, inst.IsRunnable())
		})
	}
}

func TestIsStopped(t *testing.T) {
	tests := []struct {
		name     string
		state    string
		expected bool
	}{
		{
			name:     "STOPPED is stopped",
			state:    "STOPPED",
			expected: true,
		},
		{
			name:     "SUSPENDED is stopped",
			state:    "SUSPENDED",
			expected: true,
		},
		{
			name:     "RUNNABLE is not stopped",
			state:    "RUNNABLE",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := &SQLInstance{State: tt.state}
			assert.Equal(t, tt.expected, inst.IsStopped())
		})
	}
}
