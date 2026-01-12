package filepicker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("creates file picker with default path", func(t *testing.T) {
		fp := New("", true)

		assert.NotNil(t, fp)
		assert.True(t, fp.multiSelect)
		assert.NotEmpty(t, fp.currentPath)
	})

	t.Run("creates file picker with specified path", func(t *testing.T) {
		tempDir := t.TempDir()
		fp := New(tempDir, false)

		assert.NotNil(t, fp)
		assert.Equal(t, tempDir, fp.currentPath)
		assert.False(t, fp.multiSelect)
	})
}

func TestFileEntry(t *testing.T) {
	t.Run("file entry", func(t *testing.T) {
		entry := FileEntry{
			Name:  "test.txt",
			Path:  "/path/to/test.txt",
			Size:  1024,
			IsDir: false,
		}

		item := fileItem{entry: entry}

		assert.Contains(t, item.Title(), "test.txt")
		assert.Contains(t, item.Title(), "📄")
		assert.Equal(t, "test.txt", item.FilterValue())
	})

	t.Run("directory entry", func(t *testing.T) {
		entry := FileEntry{
			Name:  "docs",
			Path:  "/path/to/docs",
			IsDir: true,
		}

		item := fileItem{entry: entry}

		assert.Contains(t, item.Title(), "docs")
		assert.Contains(t, item.Title(), "📁")
		assert.Equal(t, "Folder", item.Description())
	})

	t.Run("selected entry shows checkmark", func(t *testing.T) {
		entry := FileEntry{
			Name:     "selected.txt",
			Path:     "/path/to/selected.txt",
			Selected: true,
		}

		item := fileItem{entry: entry}

		assert.Contains(t, item.Title(), "✓")
	})
}

func TestReadDirectory(t *testing.T) {
	// Create a temp directory with some files
	tempDir := t.TempDir()

	// Create test files
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("test data"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(tempDir, "subdir"), 0755))

	fp := New(tempDir, true)
	entries, err := fp.readDirectory(tempDir)

	require.NoError(t, err)
	// Should have: "..", "subdir", "file1.txt", "file2.txt" (sorted: dirs first)
	assert.GreaterOrEqual(t, len(entries), 4)

	// First should be ".."
	assert.Equal(t, "..", entries[0].Name)
	assert.True(t, entries[0].IsDir)

	// Second should be directory "subdir"
	assert.Equal(t, "subdir", entries[1].Name)
	assert.True(t, entries[1].IsDir)
}

func TestToggleSelection(t *testing.T) {
	fp := New("", true)

	path := "/path/to/file.txt"

	// Initially not selected
	assert.False(t, fp.selected[path])

	// Toggle on
	fp.toggleSelection(path)
	assert.True(t, fp.selected[path])

	// Toggle off
	fp.toggleSelection(path)
	assert.False(t, fp.selected[path])
}

func TestGetSelectedPaths(t *testing.T) {
	fp := New("", true)

	// Add some selections
	fp.selected["/path/b.txt"] = true
	fp.selected["/path/a.txt"] = true
	fp.selected["/path/c.txt"] = true

	paths := fp.GetSelectedPaths()

	// Should be sorted
	assert.Equal(t, []string{"/path/a.txt", "/path/b.txt", "/path/c.txt"}, paths)
}

func TestSetTitle(t *testing.T) {
	fp := New("", true)

	fp.SetTitle("Custom Title")

	assert.Equal(t, "Custom Title", fp.title)
}

func TestSetSize(t *testing.T) {
	fp := New("", true)

	fp.SetSize(100, 50)

	assert.Equal(t, 100, fp.width)
	assert.Equal(t, 50, fp.height)
}

func TestGetCurrentPath(t *testing.T) {
	tempDir := t.TempDir()
	fp := New(tempDir, true)

	assert.Equal(t, tempDir, fp.GetCurrentPath())
}

func TestBuildTitle(t *testing.T) {
	fp := New("", true)
	fp.title = "Select Files"

	t.Run("without selections", func(t *testing.T) {
		title := fp.buildTitle()
		assert.Equal(t, "Select Files", title)
	})

	t.Run("with selections", func(t *testing.T) {
		fp.selected["/file1.txt"] = true
		fp.selected["/file2.txt"] = true

		title := fp.buildTitle()
		assert.Contains(t, title, "2 selected")
	})
}

func TestHiddenFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create hidden and visible files
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".hidden"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "visible.txt"), []byte(""), 0644))

	fp := New(tempDir, true)
	fp.showHidden = false

	entries, err := fp.readDirectory(tempDir)
	require.NoError(t, err)

	// Should not include hidden file
	for _, e := range entries {
		assert.NotEqual(t, ".hidden", e.Name)
	}

	// Should include visible file
	found := false
	for _, e := range entries {
		if e.Name == "visible.txt" {
			found = true
			break
		}
	}
	assert.True(t, found, "visible.txt should be in entries")
}
