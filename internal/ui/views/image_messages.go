package views

// Image action messages for communication between views and the app

// DeleteImageConfirmedMsg is sent after user confirms image deletion
type DeleteImageConfirmedMsg struct {
	ImageName string
}

// ImageActionResultMsg is sent after an image action completes
type ImageActionResultMsg struct {
	Action  string // "delete", "create_disk"
	Success bool
	Error   error
}

// DiskCreateFromImageRequestMsg requests opening the disk creation form for an image
type DiskCreateFromImageRequestMsg struct {
	ImageName string
	ImageSize int64 // Size in GB
}

// CreateDiskFromImageMsg is sent when creating a disk from an image
type CreateDiskFromImageMsg struct {
	ImageName   string
	DiskName    string
	Description string
	Zone        string
	DiskType    string // pd-standard, pd-ssd, pd-balanced
	SizeGB      int64
	Labels      map[string]string
}
