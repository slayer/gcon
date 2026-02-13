package gcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/sqladmin/v1beta4"
)

// SQLInstance is the list-view summary
type SQLInstance struct {
	Name            string
	DatabaseVersion string // e.g. "MYSQL_8_0", "POSTGRES_15"
	State           string // RUNNABLE, STOPPED, etc.
	Region          string
	Tier            string // e.g. "db-f1-micro"
	PrimaryIP       string
	CreatedAt       string
}

// SQLInstanceDetails is the full detail for the details view
type SQLInstanceDetails struct {
	Name              string
	DatabaseVersion   string
	State             string
	Region            string
	Tier              string
	PrimaryIP         string
	PrivateIP         string
	ConnectionName    string
	DiskSizeGB        int64
	DiskType          string
	AvailabilityType  string // REGIONAL, ZONAL
	StorageAutoResize bool
	BackupEnabled     bool
	BinaryLogEnabled  bool
	PITREnabled       bool
	MaintenanceWindow string // e.g. "Sunday 04:00"
	DatabaseFlags     map[string]string
	ReplicaNames      []string
	MasterInstanceName string
	IPs               []SQLIPAddress
	CreatedAt         string
	SelfLink          string
}

// SQLIPAddress represents an IP address on a SQL instance
type SQLIPAddress struct {
	IPAddress string
	Type      string // PRIMARY, PRIVATE, OUTGOING
}

// SQLDatabase represents a database within an instance
type SQLDatabase struct {
	Name      string
	Charset   string
	Collation string
}

// SQLBackupRun represents a backup run for an instance
type SQLBackupRun struct {
	ID          int64
	Status      string // SUCCESSFUL, FAILED, RUNNING, etc.
	Type        string // ON_DEMAND, AUTOMATED
	StartTime   string
	EndTime     string
	Description string
}

// SQLClient handles Cloud SQL operations
type SQLClient struct {
	service *sqladmin.Service
}

// NewSQLClient creates a new Cloud SQL client
func NewSQLClient(ctx context.Context) (*SQLClient, error) {
	service, err := sqladmin.NewService(ctx, option.WithScopes(
		sqladmin.SqlserviceAdminScope,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create sql client: %w", err)
	}

	return &SQLClient{service: service}, nil
}

// ListInstances returns all Cloud SQL instances in a project
func (c *SQLClient) ListInstances(ctx context.Context, projectID string) ([]SQLInstance, error) {
	var instances []SQLInstance

	req := c.service.Instances.List(projectID)
	for {
		resp, err := req.Context(ctx).Do()
		if err != nil {
			return nil, WrapListError(err, "sql instances", projectID)
		}

		for _, inst := range resp.Items {
			instances = append(instances, sqlInstanceFromAPI(inst))
		}

		if resp.NextPageToken == "" {
			break
		}
		req = req.PageToken(resp.NextPageToken)
	}

	sort.Slice(instances, func(i, j int) bool {
		return instances[i].Name < instances[j].Name
	})

	return instances, nil
}

// GetInstance returns detailed info for a single Cloud SQL instance
func (c *SQLClient) GetInstance(ctx context.Context, projectID, instanceName string) (*SQLInstanceDetails, error) {
	inst, err := c.service.Instances.Get(projectID, instanceName).Context(ctx).Do()
	if err != nil {
		return nil, WrapGetError(err, "sql instance", instanceName)
	}
	return sqlInstanceDetailsFromAPI(inst), nil
}

// StartInstance starts a stopped Cloud SQL instance by setting activation policy to ALWAYS
func (c *SQLClient) StartInstance(ctx context.Context, projectID, instanceName string) error {
	_, err := c.service.Instances.Patch(projectID, instanceName, &sqladmin.DatabaseInstance{
		Settings: &sqladmin.Settings{
			ActivationPolicy: "ALWAYS",
		},
	}).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "start sql instance", instanceName)
	}
	return nil
}

// StopInstance stops a running Cloud SQL instance by setting activation policy to NEVER
func (c *SQLClient) StopInstance(ctx context.Context, projectID, instanceName string) error {
	_, err := c.service.Instances.Patch(projectID, instanceName, &sqladmin.DatabaseInstance{
		Settings: &sqladmin.Settings{
			ActivationPolicy: "NEVER",
		},
	}).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "stop sql instance", instanceName)
	}
	return nil
}

// RestartInstance restarts a Cloud SQL instance
func (c *SQLClient) RestartInstance(ctx context.Context, projectID, instanceName string) error {
	_, err := c.service.Instances.Restart(projectID, instanceName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "restart sql instance", instanceName)
	}
	return nil
}

// DeleteInstance deletes a Cloud SQL instance
func (c *SQLClient) DeleteInstance(ctx context.Context, projectID, instanceName string) error {
	_, err := c.service.Instances.Delete(projectID, instanceName).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "delete sql instance", instanceName)
	}
	return nil
}

// ListDatabases returns all databases within a Cloud SQL instance
func (c *SQLClient) ListDatabases(ctx context.Context, projectID, instanceName string) ([]SQLDatabase, error) {
	resp, err := c.service.Databases.List(projectID, instanceName).Context(ctx).Do()
	if err != nil {
		return nil, WrapListError(err, "sql databases", instanceName)
	}

	databases := make([]SQLDatabase, 0, len(resp.Items))
	for _, db := range resp.Items {
		databases = append(databases, sqlDatabaseFromAPI(db))
	}

	return databases, nil
}

// ListBackupRuns returns backup runs for a Cloud SQL instance
func (c *SQLClient) ListBackupRuns(ctx context.Context, projectID, instanceName string) ([]SQLBackupRun, error) {
	var runs []SQLBackupRun

	req := c.service.BackupRuns.List(projectID, instanceName)
	for {
		resp, err := req.Context(ctx).Do()
		if err != nil {
			return nil, WrapListError(err, "sql backup runs", instanceName)
		}

		for _, run := range resp.Items {
			runs = append(runs, sqlBackupRunFromAPI(run))
		}

		if resp.NextPageToken == "" {
			break
		}
		req = req.PageToken(resp.NextPageToken)
	}

	return runs, nil
}

// CreateBackupRun triggers an on-demand backup for a Cloud SQL instance
func (c *SQLClient) CreateBackupRun(ctx context.Context, projectID, instanceName, description string) error {
	_, err := c.service.BackupRuns.Insert(projectID, instanceName, &sqladmin.BackupRun{
		Description: description,
	}).Context(ctx).Do()
	if err != nil {
		return WrapActionError(err, "create backup run", instanceName)
	}
	return nil
}

// sqlInstanceFromAPI converts an API DatabaseInstance to our list-view summary
func sqlInstanceFromAPI(inst *sqladmin.DatabaseInstance) SQLInstance {
	var primaryIP string
	for _, ip := range inst.IpAddresses {
		if ip.Type == "PRIMARY" {
			primaryIP = ip.IpAddress
			break
		}
	}

	var tier string
	if inst.Settings != nil {
		tier = inst.Settings.Tier
	}

	return SQLInstance{
		Name:            inst.Name,
		DatabaseVersion: inst.DatabaseVersion,
		State:           inst.State,
		Region:          inst.Region,
		Tier:            tier,
		PrimaryIP:       primaryIP,
		CreatedAt:       inst.CreateTime,
	}
}

// sqlInstanceDetailsFromAPI converts an API DatabaseInstance to full details
func sqlInstanceDetailsFromAPI(inst *sqladmin.DatabaseInstance) *SQLInstanceDetails {
	details := &SQLInstanceDetails{
		Name:            inst.Name,
		DatabaseVersion: inst.DatabaseVersion,
		State:           inst.State,
		Region:          inst.Region,
		ConnectionName:  inst.ConnectionName,
		ReplicaNames:    inst.ReplicaNames,
		MasterInstanceName: inst.MasterInstanceName,
		CreatedAt:       inst.CreateTime,
		SelfLink:        inst.SelfLink,
	}

	// Extract IP addresses
	for _, ip := range inst.IpAddresses {
		details.IPs = append(details.IPs, SQLIPAddress{
			IPAddress: ip.IpAddress,
			Type:      ip.Type,
		})

		// Convenience fields for primary and private IPs
		switch ip.Type {
		case "PRIMARY":
			details.PrimaryIP = ip.IpAddress
		case "PRIVATE":
			details.PrivateIP = ip.IpAddress
		}
	}

	if inst.Settings != nil {
		details.Tier = inst.Settings.Tier
		details.AvailabilityType = inst.Settings.AvailabilityType
		details.DiskType = inst.Settings.DataDiskType
		details.DiskSizeGB = inst.Settings.DataDiskSizeGb

		// StorageAutoResize is a *bool — default to false if nil
		if inst.Settings.StorageAutoResize != nil {
			details.StorageAutoResize = *inst.Settings.StorageAutoResize
		}

		// Database flags
		if len(inst.Settings.DatabaseFlags) > 0 {
			details.DatabaseFlags = make(map[string]string, len(inst.Settings.DatabaseFlags))
			for _, flag := range inst.Settings.DatabaseFlags {
				details.DatabaseFlags[flag.Name] = flag.Value
			}
		}

		// Backup configuration
		if inst.Settings.BackupConfiguration != nil {
			details.BackupEnabled = inst.Settings.BackupConfiguration.Enabled
			details.BinaryLogEnabled = inst.Settings.BackupConfiguration.BinaryLogEnabled
			details.PITREnabled = inst.Settings.BackupConfiguration.PointInTimeRecoveryEnabled
		}

		// Maintenance window (Day 1=Monday ... 7=Sunday, Hour 0-23)
		if inst.Settings.MaintenanceWindow != nil {
			day := maintenanceDayName(inst.Settings.MaintenanceWindow.Day)
			hour := inst.Settings.MaintenanceWindow.Hour
			details.MaintenanceWindow = fmt.Sprintf("%s %02d:00", day, hour)
		}
	}

	return details
}

// sqlDatabaseFromAPI converts an API Database to our domain type
func sqlDatabaseFromAPI(db *sqladmin.Database) SQLDatabase {
	return SQLDatabase{
		Name:      db.Name,
		Charset:   db.Charset,
		Collation: db.Collation,
	}
}

// sqlBackupRunFromAPI converts an API BackupRun to our domain type
func sqlBackupRunFromAPI(run *sqladmin.BackupRun) SQLBackupRun {
	return SQLBackupRun{
		ID:          run.Id,
		Status:      run.Status,
		Type:        run.Type,
		StartTime:   run.StartTime,
		EndTime:     run.EndTime,
		Description: run.Description,
	}
}

// FormatDatabaseVersion converts API version strings to human-readable format.
// Exported for formatting in views
func FormatDatabaseVersion(v string) string {
	switch {
	case strings.HasPrefix(v, "MYSQL_"):
		// "MYSQL_8_0" -> "MySQL 8.0", "MYSQL_5_7" -> "MySQL 5.7"
		version := strings.TrimPrefix(v, "MYSQL_")
		version = strings.Replace(version, "_", ".", 1)
		return "MySQL " + version
	case strings.HasPrefix(v, "POSTGRES_"):
		// "POSTGRES_15" -> "PostgreSQL 15"
		version := strings.TrimPrefix(v, "POSTGRES_")
		return "PostgreSQL " + version
	case strings.HasPrefix(v, "SQLSERVER_"):
		// "SQLSERVER_2019_STANDARD" -> "SQL Server 2019"
		// "SQLSERVER_2022_ENTERPRISE" -> "SQL Server 2022"
		parts := strings.SplitN(strings.TrimPrefix(v, "SQLSERVER_"), "_", 2)
		if len(parts) >= 1 {
			return "SQL Server " + parts[0]
		}
		return v
	default:
		return v
	}
}

// IsRunnable returns true if the Cloud SQL instance state is RUNNABLE
func (i *SQLInstance) IsRunnable() bool {
	return i.State == "RUNNABLE"
}

// IsStopped returns true if the Cloud SQL instance state is STOPPED or SUSPENDED
func (i *SQLInstance) IsStopped() bool {
	return i.State == "STOPPED" || i.State == "SUSPENDED"
}

// maintenanceDayName converts GCP maintenance window day number to name.
// GCP uses 1=Monday ... 7=Sunday, 0=unspecified.
func maintenanceDayName(day int64) string {
	switch day {
	case 1:
		return "Monday"
	case 2:
		return "Tuesday"
	case 3:
		return "Wednesday"
	case 4:
		return "Thursday"
	case 5:
		return "Friday"
	case 6:
		return "Saturday"
	case 7:
		return "Sunday"
	default:
		return "Unspecified"
	}
}
