package views

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/components/filepicker"
	"github.com/slayer/gcon/internal/ui/components/progress"
	"github.com/stretchr/testify/assert"
)

func TestObjectItem(t *testing.T) {
	t.Run("folder item", func(t *testing.T) {
		obj := gcp.StorageObject{
			Name:        "folder1/",
			DisplayName: "folder1",
			IsFolder:    true,
		}

		item := objectItem{object: obj}

		assert.Contains(t, item.Title(), "folder1")
		assert.Contains(t, item.Title(), "📁")
		assert.Contains(t, item.Description(), "Folder")
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

		item := objectItem{object: obj}

		assert.Contains(t, item.Title(), "document.pdf")
		assert.Contains(t, item.Title(), "📄")

		desc := item.Description()
		assert.Contains(t, desc, "1.5 MB")
		assert.Contains(t, desc, "application/pdf")
		assert.Contains(t, desc, "2024-01-20")
	})

	t.Run("FilterValue", func(t *testing.T) {
		obj := gcp.StorageObject{
			Name:        "image.png",
			DisplayName: "image.png",
			ContentType: "image/png",
		}

		item := objectItem{object: obj}
		filterVal := item.FilterValue()

		assert.Contains(t, filterVal, "image.png")
		assert.Contains(t, filterVal, "image/png")
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
	assert.Equal(t, 1, view.currentPage)
}

func TestObjectsViewUpdate(t *testing.T) {
	view := NewObjectsView("test-bucket", nil)

	t.Run("handles objectsLoadedMsg", func(t *testing.T) {
		objects := []gcp.StorageObject{
			{Name: "folder1/", DisplayName: "folder1", IsFolder: true},
			{Name: "file1.txt", DisplayName: "file1.txt", Size: 1024, IsFolder: false},
		}

		view.Update(objectsLoadedMsg{
			objects:   objects,
			nextToken: "next-token",
			hasMore:   true,
		})

		assert.False(t, view.loading)
		assert.Equal(t, 2, len(view.objects))
		assert.Equal(t, "next-token", view.nextPageToken)
		assert.True(t, view.hasMore)
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

	view.SetSize(100, 50)

	assert.Equal(t, 100, view.width)
	assert.Equal(t, 50, view.height)
}

func TestObjectsViewPagination(t *testing.T) {
	t.Run("resetPagination clears all pagination state", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.currentPage = 3
		view.nextPageToken = "some-token"
		view.currentPageToken = "current-token"
		view.pageTokenHistory = []string{"token1", "token2"}
		view.hasMore = true

		view.resetPagination()

		assert.Equal(t, 1, view.currentPage)
		assert.Empty(t, view.nextPageToken)
		assert.Empty(t, view.currentPageToken)
		assert.Empty(t, view.pageTokenHistory)
		assert.False(t, view.hasMore)
	})

	t.Run("NextPage updates pagination state correctly", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.currentPage = 1
		view.currentPageToken = ""
		view.nextPageToken = "page2-token"
		view.hasMore = true

		// Simulate pressing 'n' for next page
		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

		// State should be updated for loading next page
		assert.True(t, view.loading)
		assert.Equal(t, 2, view.currentPage)
		assert.Equal(t, "page2-token", view.currentPageToken)
		// History should contain the previous page token (empty for first page)
		assert.Equal(t, []string{""}, view.pageTokenHistory)
	})

	t.Run("PrevPage updates pagination state correctly", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.currentPage = 2
		view.currentPageToken = "page2-token"
		view.nextPageToken = "page3-token"
		view.pageTokenHistory = []string{""} // First page token
		view.hasMore = true

		// Simulate pressing 'p' for previous page
		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

		// State should be updated for loading previous page
		assert.True(t, view.loading)
		assert.Equal(t, 1, view.currentPage)
		assert.Equal(t, "", view.currentPageToken)
		assert.Empty(t, view.pageTokenHistory)
	})

	t.Run("NextPage does nothing when no more pages", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.currentPage = 1
		view.hasMore = false
		view.nextPageToken = ""

		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

		// State should not change
		assert.False(t, view.loading)
		assert.Equal(t, 1, view.currentPage)
	})

	t.Run("PrevPage does nothing on first page", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.currentPage = 1
		view.pageTokenHistory = []string{}

		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

		// State should not change
		assert.False(t, view.loading)
		assert.Equal(t, 1, view.currentPage)
	})

	t.Run("multi-page navigation maintains correct history", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false

		// Setup: on page 1
		view.currentPage = 1
		view.currentPageToken = ""
		view.nextPageToken = "page2-token"
		view.hasMore = true
		view.pageTokenHistory = []string{}

		// Navigate to page 2
		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		assert.Equal(t, 2, view.currentPage)
		assert.Equal(t, "page2-token", view.currentPageToken)
		assert.Equal(t, []string{""}, view.pageTokenHistory)

		// Simulate page 2 loaded with next token
		view.loading = false
		view.nextPageToken = "page3-token"

		// Navigate to page 3
		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		assert.Equal(t, 3, view.currentPage)
		assert.Equal(t, "page3-token", view.currentPageToken)
		assert.Equal(t, []string{"", "page2-token"}, view.pageTokenHistory)

		// Simulate page 3 loaded
		view.loading = false

		// Navigate back to page 2
		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
		assert.Equal(t, 2, view.currentPage)
		assert.Equal(t, "page2-token", view.currentPageToken)
		assert.Equal(t, []string{""}, view.pageTokenHistory)

		// Simulate page 2 loaded
		view.loading = false

		// Navigate back to page 1
		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
		assert.Equal(t, 1, view.currentPage)
		assert.Equal(t, "", view.currentPageToken)
		assert.Empty(t, view.pageTokenHistory)
	})
}

func TestObjectsViewFilePicker(t *testing.T) {
	t.Run("pressing u opens file picker", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.SetSize(100, 50)

		// Press 'u' to open upload file picker
		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

		assert.True(t, view.showFilePicker)
		assert.NotNil(t, view.filePicker)
	})

	t.Run("pressing u opens file picker in empty bucket", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.objects = []gcp.StorageObject{} // Empty bucket
		view.SetSize(100, 50)

		// Press 'u' to open upload file picker
		view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})

		assert.True(t, view.showFilePicker, "file picker should open in empty bucket")
		assert.NotNil(t, view.filePicker, "file picker should be created")
	})

	t.Run("file picker receives forwarded messages", func(t *testing.T) {
		view := NewObjectsView("test-bucket", nil)
		view.loading = false
		view.SetSize(100, 50)

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
		view.SetSize(100, 50)

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
		view.SetSize(100, 50)

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
		view.SetSize(100, 50)

		files := []gcp.StorageObject{
			{Name: "file1.txt", DisplayName: "file1.txt", Size: 1024},
		}

		view.Update(downloadStartMsg{files: files})

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
		view.SetSize(100, 50)

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
