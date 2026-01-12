package views

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestObjectToRow(t *testing.T) {
	t.Run("folder item", func(t *testing.T) {
		obj := gcp.StorageObject{
			Name:        "folder1/",
			DisplayName: "folder1",
			IsFolder:    true,
		}

		row := objectToRow(obj)

		assert.Contains(t, row.Data[0], "folder1")
		assert.Contains(t, row.Data[0], "📁")
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
		assert.Contains(t, row.Data[0], "📄")
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
