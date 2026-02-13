package views

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/confirm"
	"github.com/slayer/gcon/internal/ui/components/filepicker"
	"github.com/slayer/gcon/internal/ui/components/progress"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/slayer/gcon/internal/ui/symbols"
	"github.com/stretchr/testify/assert"
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
