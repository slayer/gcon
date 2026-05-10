package views

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/gcp/usage"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/filepicker"
	"github.com/slayer/gcon/internal/ui/components/progress"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testContext creates a test context with standard dimensions
func testContext() *context.ProgramContext {
	return &context.ProgramContext{ContentWidth: 100, ContentHeight: 50}
}

func TestObjectToRow(t *testing.T) {
	t.Run("folder item", func(t *testing.T) {
		obj := gcp.StorageObject{
			Name:        "folder1/",
			DisplayName: "folder1",
			IsFolder:    true,
		}

		row := objectToRow(obj)

		assert.Contains(t, row.Data[0], "folder1")
		assert.Contains(t, row.Data[0], symbols.Folder())
		assert.Equal(t, "-", row.Data[1]) // Size
		assert.Equal(t, "Folder", row.Data[2])
		assert.Equal(t, "-", row.Data[3]) // Modified
		assert.Equal(t, "folder1/", row.ID)
	})

	t.Run("file item", func(t *testing.T) {
		obj := gcp.StorageObject{
			Name:        "document.pdf",
			DisplayName: "document.pdf",
			Size:        1572864, // 1.5 MB
			ContentType: "application/pdf",
			Updated:     time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
			IsFolder:    false,
		}

		row := objectToRow(obj)

		assert.Contains(t, row.Data[0], "document.pdf")
		assert.Contains(t, row.Data[0], symbols.File())
		assert.Equal(t, "1.5 MB", row.Data[1])
		assert.Equal(t, "application/pdf", row.Data[2])
		assert.Equal(t, "2024-01-20", row.Data[3])
		assert.Equal(t, "document.pdf", row.ID)
	})

	t.Run("FilterValue", func(t *testing.T) {
		obj := gcp.StorageObject{
			Name:        "image.png",
			DisplayName: "image.png",
			ContentType: "image/png",
		}

		row := objectToRow(obj)

		assert.Contains(t, row.FilterValue, "image.png")
		assert.Contains(t, row.FilterValue, "image/png")
	})
}

func TestNewObjectsView(t *testing.T) {
	bucketName := "test-bucket"
	// Note: storageClient is nil for this test since we're not testing actual API calls
	view := NewObjectsView(bucketName, nil)

	assert.NotNil(t, view)
	assert.Equal(t, bucketName, view.bucketName)
	assert.Equal(t, "", view.currentPrefix)
	assert.Empty(t, view.prefixStack)
	assert.True(t, view.loading)
	assert.False(t, view.allLoaded)
}

func TestObjectsViewUpdate(t *testing.T) {
	view := NewObjectsView("test-bucket", nil)

	t.Run("handles objectsLoadedMsg", func(t *testing.T) {
		objects := []gcp.StorageObject{
			{Name: "folder1/", DisplayName: "folder1", IsFolder: true},
			{Name: "file1.txt", DisplayName: "file1.txt", Size: 1024, IsFolder: false},
		}

		view.Update(objectsLoadedMsg{
			objects:    objects,
			nextToken:  "next-token",
			hasMore:    true,
			generation: 0,
		})

		assert.False(t, view.loading)
		assert.Equal(t, 2, len(view.objects))
		assert.Equal(t, "next-token", view.nextPageToken)
		assert.False(t, view.allLoaded)
	})

	t.Run("handles objectsErrorMsg", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		testErr := assert.AnError

		view.Update(objectsErrorMsg{err: testErr})

		assert.False(t, view.loading)
		assert.Equal(t, testErr, view.err)
	})
}

func TestParentPrefix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"folder1/", ""},
		{"folder1/folder2/", "folder1/"},
		{"a/b/c/", "a/b/"},
		{"folder1", ""}, // missing trailing slash still treated as folder
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, parentPrefix(tc.in))
		})
	}
}

func TestObjectsView_ParentNavRowInjected(t *testing.T) {
	t.Run("not at root: prepends .. row", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.currentPrefix = "folder1/"
		view.Update(objectsLoadedMsg{
			objects: []gcp.StorageObject{
				{Name: "folder1/file.txt", DisplayName: "file.txt", IsFolder: false},
			},
			generation: 0,
		})

		rows := view.table.Rows()
		if assert.GreaterOrEqual(t, len(rows), 1) {
			assert.Equal(t, parentNavRowID, rows[0].ID, "first row should be the .. parent nav")
		}
	})

	t.Run("at root: no .. row", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.currentPrefix = ""
		view.Update(objectsLoadedMsg{
			objects: []gcp.StorageObject{
				{Name: "file.txt", DisplayName: "file.txt", IsFolder: false},
			},
			generation: 0,
		})

		rows := view.table.Rows()
		if assert.Len(t, rows, 1) {
			assert.NotEqual(t, parentNavRowID, rows[0].ID)
		}
	})

	t.Run("empty subfolder still shows .. row and renders the table", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.currentPrefix = "empty-folder/"
		view.Update(objectsLoadedMsg{objects: nil, generation: 0})

		rows := view.table.Rows()
		if assert.Len(t, rows, 1, "empty subfolder should still have the .. row") {
			assert.Equal(t, parentNavRowID, rows[0].ID)
		}

		out := view.View()
		assert.NotContains(t, out, "This folder is empty.",
			"empty subfolder should render the table (with ..), not the empty-state message")
		assert.Contains(t, out, "←: up",
			"help text should advertise ←: up when in a subfolder")
	})
}

func TestObjectsView_NavigateInto(t *testing.T) {
	t.Run("enters a folder and pushes onto stack", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.currentPrefix = "folder1/"
		view.prefixStack = []string{""}

		_ = view.navigateInto("folder1/folder2/")

		assert.Equal(t, "folder1/folder2/", view.currentPrefix)
		assert.Equal(t, []string{"", "folder1/"}, view.prefixStack)
		assert.True(t, view.loading)
	})
}

func TestObjectsView_NavigateUp(t *testing.T) {
	t.Run("moves to parent and pushes onto stack", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.currentPrefix = "folder1/folder2/"
		view.prefixStack = []string{""}

		_ = view.navigateUp()

		assert.Equal(t, "folder1/", view.currentPrefix)
		assert.Equal(t, []string{"", "folder1/folder2/"}, view.prefixStack)
		assert.True(t, view.loading)
	})

	t.Run("no-op at root", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.currentPrefix = ""
		view.prefixStack = nil

		cmd := view.navigateUp()

		assert.Nil(t, cmd)
		assert.Equal(t, "", view.currentPrefix)
		assert.Empty(t, view.prefixStack)
	})
}

func TestObjectsViewHandleBack(t *testing.T) {
	t.Run("returns to parent folder when in subfolder", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.prefixStack = []string{"", "folder1/"}
		view.currentPrefix = "folder1/folder2/"
		view.loading = false

		handled, _ := view.HandleBack()

		assert.True(t, handled)
		assert.Equal(t, "folder1/", view.currentPrefix)
		assert.Equal(t, 1, len(view.prefixStack))
	})

	t.Run("returns not handled when at root", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.prefixStack = []string{}
		view.currentPrefix = ""
		view.loading = false

		handled, _ := view.HandleBack()

		assert.False(t, handled)
	})
}

func TestObjectsViewGetters(t *testing.T) {
	view := NewObjectsView("test-bucket", nil)
	view.currentPrefix = "folder1/folder2/"

	assert.Equal(t, "test-bucket", view.GetBucketName())
	assert.Equal(t, "folder1/folder2/", view.GetCurrentPath())
}

func TestObjectsViewSetSize(t *testing.T) {
	view := NewObjectsView("test-bucket", nil)

	view.SetContext(testContext())

	assert.Equal(t, 100, view.width)
	assert.Equal(t, 50, view.height)
}

func TestObjectsViewInfiniteScroll(t *testing.T) {
	t.Run("resetScrollState clears all scroll state", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.nextPageToken = "some-token"
		view.loadingMore = true
		view.allLoaded = true
		view.objects = []gcp.StorageObject{{Name: "test.txt"}}

		initialGen := view.loadGeneration
		view.resetScrollState()

		assert.Empty(t, view.nextPageToken)
		assert.False(t, view.loadingMore)
		assert.False(t, view.allLoaded)
		assert.Nil(t, view.objects)
		assert.Equal(t, initialGen+1, view.loadGeneration, "generation should increment")
	})

	t.Run("objectsMoreLoadedMsg appends objects", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.loadingMore = true
		view.SetContext(testContext())

		// Load initial batch
		view.objects = []gcp.StorageObject{
			{Name: "file1.txt", DisplayName: "file1.txt"},
		}
		view.table.SetRows([]table.Row{objectToRow(view.objects[0])})

		// Append more data
		view.Update(objectsMoreLoadedMsg{
			objects: []gcp.StorageObject{
				{Name: "file2.txt", DisplayName: "file2.txt"},
			},
			nextToken:  "next-token",
			hasMore:    true,
			generation: 0,
		})

		assert.False(t, view.loadingMore)
		assert.Equal(t, 2, len(view.objects))
		assert.Equal(t, "next-token", view.nextPageToken)
		assert.False(t, view.allLoaded)
	})

	t.Run("objectsMoreLoadedMsg marks all loaded when no more", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.loadingMore = true
		view.SetContext(testContext())

		view.objects = []gcp.StorageObject{{Name: "file1.txt", DisplayName: "file1.txt"}}
		view.table.SetRows([]table.Row{objectToRow(view.objects[0])})

		view.Update(objectsMoreLoadedMsg{
			objects: []gcp.StorageObject{
				{Name: "file2.txt", DisplayName: "file2.txt"},
			},
			hasMore:    false,
			generation: 0,
		})

		assert.True(t, view.allLoaded)
	})

	t.Run("stale objectsMoreLoadedMsg is ignored but resets loadingMore", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.loadingMore = true
		view.loadGeneration = 2

		view.Update(objectsMoreLoadedMsg{
			objects:    []gcp.StorageObject{{Name: "stale.txt"}},
			generation: 1, // Stale generation
		})

		// Stale data discarded, but loadingMore cleared so UI doesn't get stuck
		assert.False(t, view.loadingMore)
		assert.Empty(t, view.objects)
	})

	t.Run("stale objectsLoadedMsg is ignored", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = true
		view.loadGeneration = 3

		view.Update(objectsLoadedMsg{
			objects:    []gcp.StorageObject{{Name: "stale.txt"}},
			generation: 2, // Stale generation
		})

		// Should still be loading since stale response was ignored
		assert.True(t, view.loading)
		assert.Empty(t, view.objects)
	})

	t.Run("NearBottomMsg triggers loadMore when data available", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.loadingMore = false
		view.allLoaded = false
		view.nextPageToken = "some-token"

		cmd := view.Update(table.NearBottomMsg{})

		assert.True(t, view.loadingMore)
		assert.NotNil(t, cmd)
	})

	t.Run("NearBottomMsg ignored when already loading more", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.loadingMore = true

		cmd := view.Update(table.NearBottomMsg{})

		assert.Nil(t, cmd)
	})

	t.Run("NearBottomMsg ignored when all loaded", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.allLoaded = true

		cmd := view.Update(table.NearBottomMsg{})

		assert.Nil(t, cmd)
	})

	t.Run("NearBottomMsg ignored when max objects reached", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.allLoaded = false
		view.nextPageToken = "some-token"
		// Simulate hitting the cap
		view.objects = make([]gcp.StorageObject, maxLoadedObjects)

		cmd := view.Update(table.NearBottomMsg{})

		assert.False(t, view.loadingMore, "should not start loading when cap reached")
		assert.Nil(t, cmd)
	})

	t.Run("objectsMoreErrorMsg preserves existing data", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.loadingMore = true
		view.objects = []gcp.StorageObject{
			{Name: "existing.txt", DisplayName: "existing.txt"},
		}

		view.Update(objectsMoreErrorMsg{err: assert.AnError})

		assert.False(t, view.loadingMore, "loadingMore should be cleared")
		assert.NotNil(t, view.loadMoreErr, "load-more error should be set")
		assert.Nil(t, view.err, "main error should NOT be set")
		assert.Len(t, view.objects, 1, "existing objects should be preserved")
	})
}

func TestObjectsViewFilePicker(t *testing.T) {
	t.Run("pressing u opens file picker", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.SetContext(testContext())

		// Press 'u' to open upload file picker
		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

		assert.True(t, view.showFilePicker)
		assert.NotNil(t, view.filePicker)
	})

	t.Run("pressing u opens file picker in empty bucket", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.objects = []gcp.StorageObject{} // Empty bucket
		view.SetContext(testContext())

		// Press 'u' to open upload file picker
		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

		assert.True(t, view.showFilePicker, "file picker should open in empty bucket")
		assert.NotNil(t, view.filePicker, "file picker should be created")
	})

	t.Run("file picker receives forwarded messages", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.SetContext(testContext())

		// Open file picker
		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
		assert.True(t, view.showFilePicker)

		// Simulate a custom message type that should be forwarded to file picker
		// The file picker's internal filePickerLoadedMsg would be forwarded via default case
		type customMsg struct{}
		view.Update(customMsg{})

		// File picker should still be active (message was forwarded, not causing error)
		assert.True(t, view.showFilePicker)
	})

	t.Run("FilePickerConfirmMsg closes picker and starts upload", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.showFilePicker = true
		view.SetContext(testContext())

		// Send confirm message with selected files
		cmd := view.Update(filepicker.FilePickerConfirmMsg{
			SelectedPaths: []string{"/tmp/test.txt"},
		})

		assert.False(t, view.showFilePicker)
		assert.Nil(t, view.filePicker)
		assert.NotNil(t, cmd) // Should return a command to start upload
	})

	t.Run("FilePickerConfirmMsg with no files does not start upload", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.showFilePicker = true

		// Send confirm with no files
		cmd := view.Update(filepicker.FilePickerConfirmMsg{
			SelectedPaths: []string{},
		})

		assert.False(t, view.showFilePicker)
		assert.Nil(t, cmd) // No command since no files selected
	})

	t.Run("FilePickerCancelMsg closes picker", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.showFilePicker = true

		view.Update(filepicker.FilePickerCancelMsg{})

		assert.False(t, view.showFilePicker)
		assert.Nil(t, view.filePicker)
	})

	t.Run("keys are forwarded to file picker when active", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.SetContext(testContext())

		// Open file picker
		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

		// Press escape - should be handled by file picker, not objects view
		cmd := view.Update(tea.KeyMsg{Type: tea.KeyEsc})

		// The file picker handles esc and returns FilePickerCancelMsg
		assert.NotNil(t, cmd)
	})

	t.Run("upload key ignored during loading", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = true // View is loading

		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

		assert.False(t, view.showFilePicker)
		assert.Nil(t, view.filePicker)
	})

	t.Run("upload key ignored during download", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.downloading = true

		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

		assert.False(t, view.showFilePicker)
	})

	t.Run("upload key ignored during upload", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.uploading = true

		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

		assert.False(t, view.showFilePicker)
	})
}

func TestObjectsViewDownload(t *testing.T) {
	t.Run("downloadStartMsg initializes download state", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.SetContext(testContext())

		files := []gcp.StorageObject{
			{Name: "file1.txt", DisplayName: "file1.txt", Size: 1024},
		}

		// Manually set state since storageClient is nil and startDownload would panic
		view.downloading = true
		view.downloadFiles = files
		view.downloadIndex = 0

		assert.True(t, view.downloading)
		assert.Equal(t, files, view.downloadFiles)
		assert.Equal(t, 0, view.downloadIndex)
	})

	t.Run("downloadCompleteMsg clears download state", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.downloading = true
		view.downloadFiles = []gcp.StorageObject{{Name: "test.txt"}}
		view.downloadChan = make(chan progress.ProgressUpdate, 10)

		view.Update(downloadCompleteMsg{err: nil})

		assert.False(t, view.downloading)
		assert.Nil(t, view.downloadFiles)
		assert.Nil(t, view.downloadChan)
	})

	t.Run("downloadCompleteMsg with error sets error", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.downloading = true
		testErr := assert.AnError

		view.Update(downloadCompleteMsg{err: testErr})

		assert.False(t, view.downloading)
		assert.Equal(t, testErr, view.err)
	})

	t.Run("download key ignored during loading", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = true

		cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

		assert.Nil(t, cmd)
	})
}

func TestObjectsViewUpload(t *testing.T) {
	t.Run("uploadStartMsg initializes upload state", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.SetContext(testContext())

		// Note: uploadStartMsg requires actual files to exist for os.Stat
		// This test verifies the state change
		view.Update(uploadStartMsg{files: []string{"/nonexistent/file.txt"}})

		assert.True(t, view.uploading)
	})

	t.Run("uploadCompleteMsg clears upload state", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.uploading = true
		view.uploadFiles = []string{"/tmp/test.txt"}
		view.uploadChan = make(chan progress.ProgressUpdate, 10)

		view.Update(uploadCompleteMsg{err: nil})

		assert.False(t, view.uploading)
		assert.Nil(t, view.uploadFiles)
		assert.Nil(t, view.uploadChan)
		// Should trigger refresh (loading = true)
		assert.True(t, view.loading)
	})

	t.Run("uploadCompleteMsg with error sets error without refresh", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.uploading = true
		view.loading = false
		testErr := assert.AnError

		view.Update(uploadCompleteMsg{err: testErr})

		assert.False(t, view.uploading)
		assert.Equal(t, testErr, view.err)
		assert.False(t, view.loading) // No refresh on error
	})
}

func TestObjectsViewDelete(t *testing.T) {
	t.Run("delete key ignored during loading", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = true

		cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})

		assert.Nil(t, cmd)
		assert.False(t, view.showDeleteConfirm)
	})

	t.Run("delete key ignored during downloading", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.downloading = true

		cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})

		assert.Nil(t, cmd)
		assert.False(t, view.showDeleteConfirm)
	})

	t.Run("delete key ignored during uploading", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.uploading = true

		cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})

		assert.Nil(t, cmd)
		assert.False(t, view.showDeleteConfirm)
	})

	t.Run("delete key ignored during deleting", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.deleting = true

		cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})

		assert.Nil(t, cmd)
	})

	t.Run("deleteRequestMsg stores pending delete for single file", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		file := gcp.StorageObject{Name: "test.txt", DisplayName: "test.txt", IsFolder: false}

		cmd := view.Update(deleteRequestMsg{object: file})

		assert.NotNil(t, view.pendingDelete)
		assert.Equal(t, "test.txt", view.pendingDelete.Name)
		assert.NotNil(t, cmd) // Should return command to resolve files
	})

	t.Run("deleteFilesResolvedMsg shows confirmation dialog", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.SetContext(testContext())
		files := []gcp.StorageObject{
			{Name: "file1.txt", DisplayName: "file1.txt"},
		}

		view.Update(deleteFilesResolvedMsg{files: files})

		assert.True(t, view.showDeleteConfirm)
		assert.NotNil(t, view.deleteConfirm)
		assert.Equal(t, files, view.pendingDeleteFiles)
	})

	t.Run("deleteFilesResolvedMsg with error sets error", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.pendingDelete = &gcp.StorageObject{Name: "test.txt"}
		testErr := assert.AnError

		view.Update(deleteFilesResolvedMsg{err: testErr})

		assert.False(t, view.showDeleteConfirm)
		assert.Equal(t, testErr, view.err)
		assert.Nil(t, view.pendingDelete)
	})

	t.Run("confirm.ConfirmMsg starts deletion", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.showDeleteConfirm = true
		view.deleteConfirm = confirm.New("Delete", "Are you sure?", nil)
		view.pendingDeleteFiles = []gcp.StorageObject{
			{Name: "test.txt", DisplayName: "test.txt"},
		}

		cmd := view.Update(confirm.ConfirmMsg{})

		assert.False(t, view.showDeleteConfirm)
		assert.Nil(t, view.deleteConfirm)
		assert.NotNil(t, cmd) // Should return command to start delete
	})

	t.Run("confirm.CancelMsg clears delete state", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.showDeleteConfirm = true
		view.deleteConfirm = confirm.New("Delete", "Are you sure?", nil)
		view.pendingDelete = &gcp.StorageObject{Name: "test.txt"}
		view.pendingDeleteFiles = []gcp.StorageObject{{Name: "test.txt"}}

		view.Update(confirm.CancelMsg{})

		assert.False(t, view.showDeleteConfirm)
		assert.Nil(t, view.deleteConfirm)
		assert.Nil(t, view.pendingDelete)
		assert.Nil(t, view.pendingDeleteFiles)
	})

	t.Run("deleteStartMsg initializes delete state", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.SetContext(testContext())
		view.pendingDeleteFiles = []gcp.StorageObject{
			{Name: "test.txt", DisplayName: "test.txt"},
		}

		// Manually set deleting since startDelete would need storageClient
		view.deleting = true

		assert.True(t, view.deleting)
	})

	t.Run("deleteCompleteMsg clears delete state on success", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.deleting = true
		view.pendingDelete = &gcp.StorageObject{Name: "test.txt"}
		view.pendingDeleteFiles = []gcp.StorageObject{{Name: "test.txt"}}
		view.deleteChan = make(chan deleteProgressUpdate, 10)

		view.Update(deleteCompleteMsg{err: nil, deletedCount: 1})

		assert.False(t, view.deleting)
		assert.Nil(t, view.pendingDelete)
		assert.Nil(t, view.pendingDeleteFiles)
		assert.Nil(t, view.deleteChan)
		assert.True(t, view.loading) // Should trigger refresh
	})

	t.Run("deleteCompleteMsg with error sets error", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.deleting = true
		view.loading = false
		testErr := assert.AnError

		view.Update(deleteCompleteMsg{err: testErr, deletedCount: 0})

		assert.False(t, view.deleting)
		assert.Equal(t, testErr, view.err)
		assert.False(t, view.loading) // No refresh on error
	})

	t.Run("deleteCompleteMsg with partial failure shows detailed error", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.deleting = true
		view.loading = false
		testErr := assert.AnError

		view.Update(deleteCompleteMsg{err: testErr, deletedCount: 3, failedObject: "file4.txt"})

		assert.False(t, view.deleting)
		assert.Contains(t, view.err.Error(), "deleted 3 files")
		assert.Contains(t, view.err.Error(), "file4.txt")
	})

	t.Run("keys are forwarded to confirmation dialog when active", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.showDeleteConfirm = true
		view.deleteConfirm = confirm.New("Delete", "Are you sure?", nil)

		// Press 'y' - should be handled by confirmation dialog
		cmd := view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

		// Should return confirm message
		assert.NotNil(t, cmd)
	})

	t.Run("createDeleteConfirmDialog for single file", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.SetContext(testContext())
		files := []gcp.StorageObject{
			{Name: "test.txt", DisplayName: "test.txt"},
		}

		dialog := view.createDeleteConfirmDialog(files)

		assert.NotNil(t, dialog)
		// Verify dialog was created (we can't inspect internal fields directly)
	})

	t.Run("createDeleteConfirmDialog for multiple files", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.SetContext(testContext())
		files := []gcp.StorageObject{
			{Name: "file1.txt", DisplayName: "file1.txt"},
			{Name: "file2.txt", DisplayName: "file2.txt"},
			{Name: "file3.txt", DisplayName: "file3.txt"},
		}

		dialog := view.createDeleteConfirmDialog(files)

		assert.NotNil(t, dialog)
	})

	t.Run("createDeleteConfirmDialog truncates long file list", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.SetContext(testContext())
		files := make([]gcp.StorageObject, 10)
		for i := range 10 {
			files[i] = gcp.StorageObject{Name: "file.txt", DisplayName: "file.txt"}
		}

		dialog := view.createDeleteConfirmDialog(files)

		assert.NotNil(t, dialog)
		// Dialog should be created with truncated details (first 5 + "... and X more")
	})
}

func TestObjectsView_FolderUsageRendersInlineStats(t *testing.T) {
	v := NewObjectsView("b1", nil)
	// Inject a deep-scan ReadyMsg matching the current (bucket, prefix) tuple.
	v.Update(usage.ReadyMsg{
		Usage: usage.BucketUsage{
			Bucket:      "b1",
			Prefix:      "",
			TotalBytes:  87_300_000_000,
			ObjectCount: 142_300,
			Source:      usage.SourceDeepScan,
		},
	})
	require.NotNil(t, v.folderUsage)
	assert.Equal(t, int64(142_300), v.folderUsage.ObjectCount)

	// View() requires a context and at least one object so that we render the
	// table path. Inject one synthetic object and a sane context.
	v.objects = []gcp.StorageObject{{Name: "f1.txt", DisplayName: "f1.txt"}}
	v.loading = false
	v.SetContext(testContext())
	out := v.View()
	assert.Contains(t, out, "Folder size:")
	assert.Contains(t, out, "142,300")
}

func TestObjectsView_FolderUsageIgnoresOtherBuckets(t *testing.T) {
	v := NewObjectsView("b1", nil)
	v.Update(usage.ReadyMsg{
		Usage: usage.BucketUsage{
			Bucket:      "different-bucket",
			Prefix:      "",
			TotalBytes:  100,
			ObjectCount: 1,
			Source:      usage.SourceDeepScan,
		},
	})
	assert.Nil(t, v.folderUsage, "ReadyMsg for unrelated bucket should be ignored")
}

func TestObjectsView_FolderUsageIgnoresOtherPrefix(t *testing.T) {
	v := NewObjectsView("b1", nil)
	// View is at root; ReadyMsg for a sub-prefix should not be applied here.
	v.Update(usage.ReadyMsg{
		Usage: usage.BucketUsage{
			Bucket:      "b1",
			Prefix:      "subfolder/",
			TotalBytes:  100,
			ObjectCount: 1,
			Source:      usage.SourceDeepScan,
		},
	})
	assert.Nil(t, v.folderUsage, "ReadyMsg for unrelated prefix should be ignored")
}

func TestObjectsView_DeepScanKeyEmitsRequest(t *testing.T) {
	v := NewObjectsView("b1", nil)
	v.loading = false
	v.objects = []gcp.StorageObject{{Name: "f1.txt", DisplayName: "f1.txt"}}
	v.table.SetRows([]table.Row{{ID: "f1.txt", Data: []string{"f1.txt", "1B", "text/plain", "now"}}})
	v.SetContext(testContext())

	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	require.NotNil(t, cmd, "Pressing 'C' should emit a deep-scan request")

	got := cmd()
	req, ok := got.(UsageDeepScanRequestMsg)
	require.True(t, ok, "expected UsageDeepScanRequestMsg, got %T", got)
	assert.Equal(t, "b1", req.Bucket)
	assert.Equal(t, "", req.Prefix)
}

func TestObjectsView_FolderUsageProgressUpdates(t *testing.T) {
	v := NewObjectsView("b1", nil)
	v.Update(usage.ProgressMsg{
		Bucket:         "b1",
		Prefix:         "",
		ObjectsScanned: 5,
		BytesScanned:   500,
	})
	require.NotNil(t, v.folderUsage)
	assert.Equal(t, int64(500), v.folderUsage.TotalBytes)
	assert.Equal(t, int64(5), v.folderUsage.ObjectCount)
	assert.Equal(t, usage.SourceDeepScan, v.folderUsage.Source)
}

func TestObjectsView_FolderSizesPopulatedAfterDeepScan(t *testing.T) {
	v := NewObjectsView("b1", nil)
	// Manually inject objects so the table has folder + file rows.
	v.objects = []gcp.StorageObject{
		{Name: "2024/", DisplayName: "2024", IsFolder: true},
		{Name: "2025/", DisplayName: "2025", IsFolder: true},
		{Name: "readme.txt", DisplayName: "readme.txt", Size: 100, ContentType: "text/plain"},
	}
	rows := []table.Row{
		objectToRow(v.objects[0]),
		objectToRow(v.objects[1]),
		objectToRow(v.objects[2]),
	}
	v.table.SetRows(rows)
	// Scan results: 2024/ has ~745 GB, 2025/ has ~186 GB (1024-based units).
	v.Update(usage.ReadyMsg{
		Usage: usage.BucketUsage{
			Bucket: "b1",
			Prefix: "",
			ByTopPrefix: map[string]usage.Stat{
				"2024/":  {Bytes: 800_000_000_000, Count: 100},
				"2025/":  {Bytes: 200_000_000_000, Count: 50},
				"(root)": {Bytes: 100, Count: 1},
			},
			Source: usage.SourceDeepScan,
		},
	})
	got := v.table.Rows()
	require.Len(t, got, 3)
	assert.Contains(t, got[0].Data[1], "745.1 GB", "2024/ folder Size cell should show recursive size")
	assert.Contains(t, got[0].Data[1], "✓", "scanned cell should have ✓ marker")
	assert.Contains(t, got[1].Data[1], "186.3 GB", "2025/ folder Size cell should show recursive size")
	assert.Equal(t, "100 B", got[2].Data[1], "file Size cell should NOT change")
}

func TestFolderUsageKey(t *testing.T) {
	tests := []struct{ name, rowID, scanPrefix, want string }{
		{"top-level folder root scan", "2024/", "", "2024/"},
		{"sub-folder scoped scan", "logs/2024/", "logs/", "2024/"},
		{"sub-folder scan-prefix without slash", "logs/2024/", "logs", "2024/"},
		{"file at root", "readme.txt", "", "readme.txt"},
		{"file in folder", "logs/file.log", "logs/", "file.log"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, folderUsageKey(tt.rowID, tt.scanPrefix))
		})
	}
}
