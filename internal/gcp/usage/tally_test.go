package usage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/slayer/gcon/internal/gcp"
)

func TestTopPrefixSegment(t *testing.T) {
	tests := []struct {
		name, fullName, scanPrefix, want string
	}{
		{"root no slash", "README.md", "", "(root)"},
		{"top folder", "logs/2025/file.log", "", "logs/"},
		{"deep file scan-root", "a/b/c/d.txt", "", "a/"},
		{"prefix-scoped strips prefix", "logs/2025/01/file.log", "logs/", "2025/"},
		{"prefix-scoped root", "logs/file.log", "logs/", "(root)"},
		{"prefix without trailing slash", "logs/file.log", "logs", "(root)"},
		{"object equals prefix", "logs/", "logs/", "(root)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := topPrefixSegment(tt.fullName, tt.scanPrefix)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtensionOf(t *testing.T) {
	tests := []struct {
		name, fullName, want string
	}{
		{"simple", "file.txt", ".txt"},
		{"upper to lower", "IMAGE.JPG", ".jpg"},
		{"double extension keeps last", "archive.tar.gz", ".gz"},
		{"no extension", "Makefile", "(none)"},
		{"hidden no extension", ".bashrc", "(none)"},
		{"hidden with extension", ".env.local", ".local"},
		{"folder name", "logs/", "(none)"},
		{"path with extension", "logs/2025/file.parquet", ".parquet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extensionOf(tt.fullName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTallyObjects(t *testing.T) {
	now := time.Now()
	objs := []gcp.StorageObject{
		{Name: "logs/a.log", Size: 100, ContentType: "text/plain", Updated: now},
		{Name: "logs/b.log", Size: 200, ContentType: "text/plain", Updated: now},
		{Name: "exports/data.parquet", Size: 1_000_000, ContentType: "application/octet-stream", Updated: now},
		{Name: "README.md", Size: 50, ContentType: "text/markdown", Updated: now},
	}

	// Pretend storage class came from object metadata; tally takes it via a parallel
	// slice so we don't need to extend StorageObject for v1.
	classes := []string{"STANDARD", "STANDARD", "NEARLINE", "STANDARD"}

	usage := tallyObjects("my-bucket", "", objs, classes)

	assert.Equal(t, int64(1_000_350), usage.TotalBytes)
	assert.Equal(t, int64(4), usage.ObjectCount)
	assert.Equal(t, Stat{Bytes: 350, Count: 3}, usage.ByStorageClass["STANDARD"])
	assert.Equal(t, Stat{Bytes: 1_000_000, Count: 1}, usage.ByStorageClass["NEARLINE"])
	assert.Equal(t, Stat{Bytes: 300, Count: 2}, usage.ByTopPrefix["logs/"])
	assert.Equal(t, Stat{Bytes: 1_000_000, Count: 1}, usage.ByTopPrefix["exports/"])
	assert.Equal(t, Stat{Bytes: 50, Count: 1}, usage.ByTopPrefix["(root)"])
	assert.Equal(t, Stat{Bytes: 300, Count: 2}, usage.ByExtension[".log"])
	assert.Equal(t, Stat{Bytes: 1_000_000, Count: 1}, usage.ByExtension[".parquet"])
	assert.Equal(t, Stat{Bytes: 50, Count: 1}, usage.ByExtension[".md"])
	assert.Equal(t, SourceDeepScan, usage.Source)
	assert.Equal(t, "my-bucket", usage.Bucket)
}

func TestTallyObjects_PrefixScoped(t *testing.T) {
	objs := []gcp.StorageObject{
		{Name: "logs/2025/jan/a.log", Size: 100},
		{Name: "logs/2025/feb/b.log", Size: 200},
		{Name: "logs/2024/old.log", Size: 50},
	}
	classes := []string{"STANDARD", "STANDARD", "STANDARD"}
	usage := tallyObjects("my-bucket", "logs/", objs, classes)

	assert.Equal(t, int64(350), usage.TotalBytes)
	assert.Equal(t, "logs/", usage.Prefix)
	assert.Equal(t, Stat{Bytes: 300, Count: 2}, usage.ByTopPrefix["2025/"])
	assert.Equal(t, Stat{Bytes: 50, Count: 1}, usage.ByTopPrefix["2024/"])
}
