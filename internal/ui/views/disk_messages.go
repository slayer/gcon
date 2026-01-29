package views

// Disk action messages for communication between views and the app

// DeleteDiskRequestMsg is sent when delete disk action is triggered
// Contains the disk info for confirmation dialog
type DeleteDiskRequestMsg struct {
	DiskName string
	Zone     string
}

// DeleteDiskConfirmedMsg is sent after user confirms disk deletion
type DeleteDiskConfirmedMsg struct {
	DiskName string
	Zone     string
}

// SnapshotCreateRequestMsg requests opening the snapshot creation form
type SnapshotCreateRequestMsg struct {
	DiskName   string
	Zone       string
	AttachedTo string // Instance name if attached, empty otherwise
}

// SnapshotCreateCanceledMsg indicates user canceled snapshot creation
type SnapshotCreateCanceledMsg struct{}

// CreateSnapshotFromDiskMsg is sent when creating a snapshot from a disk
type CreateSnapshotFromDiskMsg struct {
	DiskName        string
	Zone            string
	SnapshotName    string
	Description     string
	Labels          map[string]string
	StorageLocation string // Regional or multi-regional location
}

// ImageCreateRequestMsg requests opening the image creation form
type ImageCreateRequestMsg struct {
	DiskName   string
	Zone       string
	AttachedTo string // Instance name if attached, empty otherwise
}

// ImageCreateCanceledMsg indicates user canceled image creation
type ImageCreateCanceledMsg struct{}

// CreateImageFromDiskMsg is sent when creating an image from a disk
type CreateImageFromDiskMsg struct {
	DiskName        string
	Zone            string
	ImageName       string
	Description     string
	Family          string
	Labels          map[string]string
	StorageLocation string // Regional or multi-regional location
	ForceCreate     bool   // If true, create image even if disk is attached to running instance
}

// DiskActionResultMsg is sent after a disk action completes
type DiskActionResultMsg struct {
	Action  string // "delete", "snapshot", "image"
	Success bool
	Error   error
}
