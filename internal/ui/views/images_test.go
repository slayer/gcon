package views

import (
	"testing"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/stretchr/testify/assert"
)

func TestFormatArchiveSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "-",
		},
		{
			name:     "bytes only",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "kilobytes",
			bytes:    1536, // 1.5 KB
			expected: "1.5 KB",
		},
		{
			name:     "megabytes",
			bytes:    5242880, // 5 MB
			expected: "5.0 MB",
		},
		{
			name:     "gigabytes",
			bytes:    3221225472, // 3 GB
			expected: "3.0 GB",
		},
		{
			name:     "terabytes",
			bytes:    1099511627776, // 1 TB
			expected: "1.0 TB",
		},
		{
			name:     "large terabytes capped",
			bytes:    1125899906842624, // 1 PB - function caps at TB
			expected: "1.0 TB",         // Since exp is capped at len(units)-1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatArchiveSize(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestImageStatusIcon(t *testing.T) {
	tests := []struct {
		name  string
		image gcp.Image
	}{
		{
			name: "ready image",
			image: gcp.Image{
				Status: "READY",
			},
		},
		{
			name: "failed image",
			image: gcp.Image{
				Status: "FAILED",
			},
		},
		{
			name: "pending image",
			image: gcp.Image{
				Status: "PENDING",
			},
		},
		{
			name: "deleting image",
			image: gcp.Image{
				Status: "DELETING",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := imageStatusIcon(tt.image)
			// Icon should return a non-empty string
			assert.NotEmpty(t, result)
		})
	}
}

func TestImageToRow(t *testing.T) {
	tests := []struct {
		name     string
		image    gcp.Image
		validate func(t *testing.T, row table.Row)
	}{
		{
			name: "basic image with all fields",
			image: gcp.Image{
				Name:             "my-image",
				Status:           "READY",
				Family:           "debian-11",
				DiskSizeGB:       10,
				ArchiveSizeBytes: 5242880, // 5 MB
				CreatedBy:        "my-project",
				StorageLocations: []string{"us-central1"},
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Contains(t, row.Data[0], "my-image")
				// Name should contain both icon and image name
				assert.NotEqual(t, "my-image", row.Data[0]) // Should have icon prefix
				assert.Equal(t, "my-project", row.Data[1])
				assert.Equal(t, "us-central1", row.Data[2])
				assert.Equal(t, "10 GB", row.Data[3])
				assert.Equal(t, "5.0 MB", row.Data[4])
				assert.Equal(t, "debian-11", row.Data[5])
				assert.Equal(t, "my-image", row.ID)
				assert.Contains(t, row.FilterValue, "my-image")
				assert.Contains(t, row.FilterValue, "debian-11")
				assert.Contains(t, row.FilterValue, "my-project")
			},
		},
		{
			name: "image with no storage locations",
			image: gcp.Image{
				Name:             "test-image",
				Status:           "READY",
				Family:           "ubuntu-2004",
				DiskSizeGB:       20,
				ArchiveSizeBytes: 0,
				CreatedBy:        "test-project",
				StorageLocations: []string{},
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Contains(t, row.Data[0], "test-image")
				assert.Equal(t, "test-project", row.Data[1])
				assert.Equal(t, "-", row.Data[2]) // No location
				assert.Equal(t, "20 GB", row.Data[3])
				assert.Equal(t, "-", row.Data[4]) // No archive size
				assert.Equal(t, "ubuntu-2004", row.Data[5])
			},
		},
		{
			name: "failed image",
			image: gcp.Image{
				Name:             "failed-image",
				Status:           "FAILED",
				Family:           "",
				DiskSizeGB:       5,
				ArchiveSizeBytes: 1024,
				CreatedBy:        "project-123",
				StorageLocations: []string{"us-east1"},
			},
			validate: func(t *testing.T, row table.Row) {
				assert.Contains(t, row.Data[0], "failed-image")
				// Should have status icon prefix
				assert.NotEqual(t, "failed-image", row.Data[0])
				assert.Equal(t, "project-123", row.Data[1])
				assert.Equal(t, "us-east1", row.Data[2])
				assert.Equal(t, "5 GB", row.Data[3])
				assert.Equal(t, "1.0 KB", row.Data[4])
				assert.Equal(t, "", row.Data[5]) // No family
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := imageToRow(tt.image)
			tt.validate(t, row)
		})
	}
}
