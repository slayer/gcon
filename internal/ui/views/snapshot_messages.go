package views

// Snapshot action messages for communication between views and the app

// DeleteSnapshotConfirmedMsg is sent after user confirms snapshot deletion
type DeleteSnapshotConfirmedMsg struct {
	SnapshotName string
}

// SnapshotActionResultMsg is sent after a snapshot action completes
type SnapshotActionResultMsg struct {
	Action  string // "delete", "create_disk"
	Success bool
	Error   error
}

// DiskCreateFromSnapshotRequestMsg requests opening the disk creation form
type DiskCreateFromSnapshotRequestMsg struct {
	SnapshotName string
	SnapshotSize int64 // Size in GB
}

// DiskCreateCanceledMsg indicates user canceled disk creation
type DiskCreateCanceledMsg struct{}

// CreateDiskFromSnapshotMsg is sent when creating a disk from a snapshot
type CreateDiskFromSnapshotMsg struct {
	SnapshotName string
	DiskName     string
	Description  string
	Zone         string
	DiskType     string // pd-standard, pd-ssd, pd-balanced
	SizeGB       int64
	Labels       map[string]string
}

// ImageCreateFromSnapshotRequestMsg requests opening the image creation form for a snapshot
type ImageCreateFromSnapshotRequestMsg struct {
	SnapshotName string
}

// ImageCreateFromSnapshotCanceledMsg indicates user canceled image creation from snapshot
type ImageCreateFromSnapshotCanceledMsg struct{}

// CreateImageFromSnapshotMsg is sent when creating an image from a snapshot
type CreateImageFromSnapshotMsg struct {
	SnapshotName    string
	ImageName       string
	Description     string
	Family          string
	Labels          map[string]string
	StorageLocation string // Regional or multi-regional location
}
