package views

import (
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
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
		assert.Equal(t, "next-token", view.pageToken)
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
	view := NewObjectsView("test-bucket", nil)

	t.Run("resetPagination clears all pagination state", func(t *testing.T) {
		view.currentPage = 3
		view.pageToken = "some-token"
		view.prevPageTokens = []string{"token1", "token2"}
		view.hasMore = true

		view.resetPagination()

		assert.Equal(t, 1, view.currentPage)
		assert.Empty(t, view.pageToken)
		assert.Empty(t, view.prevPageTokens)
		assert.False(t, view.hasMore)
	})
}
