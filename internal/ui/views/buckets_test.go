package views

import (
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/stretchr/testify/assert"
)

func TestBucketItem(t *testing.T) {
	bucket := gcp.Bucket{
		Name:         "test-bucket",
		Location:     "us-central1",
		StorageClass: "STANDARD",
		Created:      time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}

	item := bucketItem{bucket: bucket}

	t.Run("Title", func(t *testing.T) {
		assert.Contains(t, item.Title(), "test-bucket")
	})

	t.Run("Description", func(t *testing.T) {
		desc := item.Description()
		assert.Contains(t, desc, "us-central1")
		assert.Contains(t, desc, "STANDARD")
		assert.Contains(t, desc, "2024-01-15")
	})

	t.Run("FilterValue", func(t *testing.T) {
		filterVal := item.FilterValue()
		assert.Contains(t, filterVal, "test-bucket")
		assert.Contains(t, filterVal, "us-central1")
		assert.Contains(t, filterVal, "STANDARD")
	})
}

func TestNewBucketsView(t *testing.T) {
	projectID := "test-project"
	view := NewBucketsView(projectID)

	assert.NotNil(t, view)
	assert.Equal(t, projectID, view.projectID)
	assert.True(t, view.loading)
	assert.Nil(t, view.storageClient)
	assert.Empty(t, view.buckets)
}

func TestBucketsViewUpdate(t *testing.T) {
	projectID := "test-project"
	view := NewBucketsView(projectID)

	t.Run("handles bucketsLoadedMsg", func(t *testing.T) {
		buckets := []gcp.Bucket{
			{Name: "bucket1", Location: "us-central1", StorageClass: "STANDARD"},
			{Name: "bucket2", Location: "europe-west1", StorageClass: "NEARLINE"},
		}

		view.Update(bucketsLoadedMsg{buckets: buckets})

		assert.False(t, view.loading)
		assert.Equal(t, 2, len(view.buckets))
		assert.Equal(t, "bucket1", view.buckets[0].Name)
		assert.Equal(t, "bucket2", view.buckets[1].Name)
	})

	t.Run("handles bucketsErrorMsg", func(t *testing.T) {
		view := NewBucketsView(projectID)
		testErr := assert.AnError

		view.Update(bucketsErrorMsg{err: testErr})

		assert.False(t, view.loading)
		assert.Equal(t, testErr, view.err)
	})
}

func TestBucketsViewSetSize(t *testing.T) {
	view := NewBucketsView("test-project")

	view.SetSize(100, 50)

	assert.Equal(t, 100, view.width)
	assert.Equal(t, 50, view.height)
}
