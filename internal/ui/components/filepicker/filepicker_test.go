package filepicker

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/ui/symbols"
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
		assert.Contains(t, item.Title(), symbols.File())
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
		assert.Contains(t, item.Title(), symbols.Folder())
		assert.Contains(t, item.Title(), "[ ]") // Checkbox for unselected
		assert.Equal(t, "", item.Description()) // Single-line display, no description
	})

	t.Run("selected entry shows checkbox checked", func(t *testing.T) {
		entry := FileEntry{
			Name:     "selected.txt",
			Path:     "/path/to/selected.txt",
			Selected: true,
		}

		item := fileItem{entry: entry}

		assert.Contains(t, item.Title(), "[x]") // Checked checkbox
	})

	t.Run("parent directory entry has no checkbox", func(t *testing.T) {
		entry := FileEntry{
			Name:  "..",
			Path:  "/path/to",
			IsDir: true,
		}

		item := fileItem{entry: entry}

		assert.NotContains(t, item.Title(), "[")   // No checkbox
		assert.NotContains(t, item.Title(), "[x]") // No checkbox
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

func TestFilePickerUpdate(t *testing.T) {
	t.Run("handles filePickerLoadedMsg", func(t *testing.T) {
		tempDir := t.TempDir()
		fp := New(tempDir, true)

		entries := []FileEntry{
			{Name: "..", Path: filepath.Dir(tempDir), IsDir: true},
			{Name: "test.txt", Path: filepath.Join(tempDir, "test.txt"), Size: 100},
		}

		fp.Update(filePickerLoadedMsg{entries: entries})

		assert.Equal(t, entries, fp.entries)
		// List should have items now
		assert.Equal(t, 2, len(fp.list.Items()))
	})

	t.Run("handles filePickerErrorMsg", func(t *testing.T) {
		tempDir := t.TempDir()
		fp := New(tempDir, true)
		testErr := assert.AnError

		fp.Update(filePickerErrorMsg{err: testErr})

		assert.Equal(t, testErr, fp.err)
	})

	t.Run("Init returns loadDirectory command", func(t *testing.T) {
		tempDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("test"), 0644))

		fp := New(tempDir, true)
		cmd := fp.Init()

		assert.NotNil(t, cmd)

		// Execute the command and check result
		msg := cmd()
		loadedMsg, ok := msg.(filePickerLoadedMsg)
		assert.True(t, ok, "Init should return filePickerLoadedMsg")
		assert.NotEmpty(t, loadedMsg.entries)
	})

	t.Run("Cancel key returns FilePickerCancelMsg", func(t *testing.T) {
		tempDir := t.TempDir()
		fp := New(tempDir, true)

		cmd := fp.Update(tea.KeyMsg{Type: tea.KeyEsc})

		assert.NotNil(t, cmd)
		msg := cmd()
		_, ok := msg.(FilePickerCancelMsg)
		assert.True(t, ok)
	})

	t.Run("Backspace navigates to parent directory", func(t *testing.T) {
		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "subdir")
		require.NoError(t, os.Mkdir(subDir, 0755))

		fp := New(subDir, true)

		cmd := fp.Update(tea.KeyMsg{Type: tea.KeyBackspace})

		assert.NotNil(t, cmd)
		assert.Equal(t, tempDir, fp.currentPath)
	})

	t.Run("Left arrow on first page navigates to parent directory", func(t *testing.T) {
		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "subdir")
		require.NoError(t, os.Mkdir(subDir, 0755))

		fp := New(subDir, true)

		// Load directory to initialize list
		entries, _ := fp.readDirectory(subDir)
		fp.Update(filePickerLoadedMsg{entries: entries})

		// Should be on first page (page 0)
		assert.Equal(t, 0, fp.list.Paginator.Page)

		// Left arrow should navigate up when on first page
		cmd := fp.Update(tea.KeyMsg{Type: tea.KeyLeft})

		assert.NotNil(t, cmd)
		assert.Equal(t, tempDir, fp.currentPath)
	})

	t.Run("Space toggles selection in multi-select mode", func(t *testing.T) {
		tempDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("test"), 0644))

		fp := New(tempDir, true) // multi-select enabled

		// Load directory first
		entries, _ := fp.readDirectory(tempDir)
		fp.Update(filePickerLoadedMsg{entries: entries})

		// Find the file entry (not "..")
		var fileEntry FileEntry
		for _, e := range fp.entries {
			if e.Name == "file.txt" {
				fileEntry = e
				break
			}
		}

		// Select the file in the list
		for i, item := range fp.list.Items() {
			if fi, ok := item.(fileItem); ok && fi.entry.Name == "file.txt" {
				fp.list.Select(i)
				break
			}
		}

		// Toggle selection
		fp.Update(tea.KeyMsg{Type: tea.KeySpace})

		assert.True(t, fp.selected[fileEntry.Path])

		// Toggle again
		fp.Update(tea.KeyMsg{Type: tea.KeySpace})

		assert.False(t, fp.selected[fileEntry.Path])
	})

	t.Run("SelectAll selects all entries except parent", func(t *testing.T) {
		tempDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("test"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("test"), 0644))

		fp := New(tempDir, true)

		// Load directory
		entries, _ := fp.readDirectory(tempDir)
		fp.Update(filePickerLoadedMsg{entries: entries})

		// Press 'a' to select all
		fp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

		// ".." should not be selected, but files should be
		assert.GreaterOrEqual(t, len(fp.selected), 2)
		for path := range fp.selected {
			assert.NotEqual(t, filepath.Dir(tempDir), path)
		}
	})

	t.Run("DeselectAll clears all selections", func(t *testing.T) {
		tempDir := t.TempDir()
		fp := New(tempDir, true)

		fp.selected["/path/file1.txt"] = true
		fp.selected["/path/file2.txt"] = true

		// Press 'A' (shift+a) to deselect all
		fp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})

		assert.Empty(t, fp.selected)
	})

	t.Run("Enter on directory navigates into it", func(t *testing.T) {
		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "subdir")
		require.NoError(t, os.Mkdir(subDir, 0755))

		fp := New(tempDir, true)

		// Load directory
		entries, _ := fp.readDirectory(tempDir)
		fp.Update(filePickerLoadedMsg{entries: entries})

		// Find and select the subdir
		for i, item := range fp.list.Items() {
			if fi, ok := item.(fileItem); ok && fi.entry.Name == "subdir" {
				fp.list.Select(i)
				break
			}
		}

		// Press enter to navigate into subdir
		cmd := fp.Update(tea.KeyMsg{Type: tea.KeyEnter})

		assert.NotNil(t, cmd)
		assert.Equal(t, subDir, fp.currentPath)
	})

	t.Run("Enter on file with selections confirms", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "file.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("test"), 0644))

		fp := New(tempDir, true)

		// Load directory
		entries, _ := fp.readDirectory(tempDir)
		fp.Update(filePickerLoadedMsg{entries: entries})

		// Pre-select a file
		fp.selected[filePath] = true

		// Find and select the file in the list
		for i, item := range fp.list.Items() {
			if fi, ok := item.(fileItem); ok && fi.entry.Name == "file.txt" {
				fp.list.Select(i)
				break
			}
		}

		// Press enter
		cmd := fp.Update(tea.KeyMsg{Type: tea.KeyEnter})

		assert.NotNil(t, cmd)
		msg := cmd()
		confirmMsg, ok := msg.(FilePickerConfirmMsg)
		assert.True(t, ok)
		assert.Contains(t, confirmMsg.SelectedPaths, filePath)
	})

	t.Run("Navigate up selects previous folder", func(t *testing.T) {
		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "mysubdir")
		require.NoError(t, os.Mkdir(subDir, 0755))

		// Start in subdir
		fp := New(subDir, true)

		// Navigate up using backspace
		cmd := fp.Update(tea.KeyMsg{Type: tea.KeyBackspace})

		assert.NotNil(t, cmd)
		assert.Equal(t, tempDir, fp.currentPath)

		// Execute the returned command to get the loadedMsg
		msg := cmd()
		loadedMsg, ok := msg.(filePickerLoadedMsg)
		assert.True(t, ok)

		// The selectTarget should be "mysubdir"
		assert.Equal(t, "mysubdir", loadedMsg.selectTarget)
	})

	t.Run("Navigate up via .. entry selects previous folder", func(t *testing.T) {
		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "anothersubdir")
		require.NoError(t, os.Mkdir(subDir, 0755))

		// Start in subdir
		fp := New(subDir, true)

		// Load directory first
		entries, _ := fp.readDirectory(subDir)
		fp.Update(filePickerLoadedMsg{entries: entries})

		// Select ".." entry
		for i, item := range fp.list.Items() {
			if fi, ok := item.(fileItem); ok && fi.entry.Name == ".." {
				fp.list.Select(i)
				break
			}
		}

		// Press enter on ".."
		cmd := fp.Update(tea.KeyMsg{Type: tea.KeyEnter})

		assert.NotNil(t, cmd)
		assert.Equal(t, tempDir, fp.currentPath)

		// Execute the returned command to get the loadedMsg
		msg := cmd()
		loadedMsg, ok := msg.(filePickerLoadedMsg)
		assert.True(t, ok)

		// The selectTarget should be "anothersubdir"
		assert.Equal(t, "anothersubdir", loadedMsg.selectTarget)
	})

	t.Run("filePickerLoadedMsg with selectTarget selects correct entry", func(t *testing.T) {
		tempDir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(tempDir, "aaa"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(tempDir, "bbb"), 0755))
		require.NoError(t, os.Mkdir(filepath.Join(tempDir, "ccc"), 0755))

		fp := New(tempDir, true)

		// Load directory with selectTarget = "bbb"
		entries, _ := fp.readDirectory(tempDir)
		fp.Update(filePickerLoadedMsg{entries: entries, selectTarget: "bbb"})

		// Check that "bbb" is selected in the list
		selectedItem := fp.list.SelectedItem()
		assert.NotNil(t, selectedItem)

		fi, ok := selectedItem.(fileItem)
		assert.True(t, ok)
		assert.Equal(t, "bbb", fi.entry.Name)
	})
}

func TestFilePickerView(t *testing.T) {
	t.Run("renders error when set", func(t *testing.T) {
		fp := New("", true)
		fp.err = assert.AnError

		view := fp.View()

		assert.Contains(t, view, "Error")
	})

	t.Run("renders current path", func(t *testing.T) {
		tempDir := t.TempDir()
		fp := New(tempDir, true)
		fp.SetSize(100, 30)

		// Load entries
		entries, _ := fp.readDirectory(tempDir)
		fp.Update(filePickerLoadedMsg{entries: entries})

		view := fp.View()

		// Path should be displayed (or truncated version)
		assert.NotEmpty(t, view)
	})
}
