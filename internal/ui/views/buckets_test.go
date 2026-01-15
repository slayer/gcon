package views

import (
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
)

func TestBucketToRow(t *testing.T) {
	bucket := gcp.Bucket{
		Name:         "test-bucket",
		Location:     "us-central1",
		StorageClass: "STANDARD",
		Created:      time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}

	row := bucketToRow(bucket)

	t.Run("Data contains bucket info", func(t *testing.T) {
		assert.Contains(t, row.Data[0], "test-bucket")
		assert.Equal(t, "us-central1", row.Data[1])
		assert.Equal(t, "STANDARD", row.Data[2])
		assert.Equal(t, "2024-01-15", row.Data[3])
	})

	t.Run("FilterValue", func(t *testing.T) {
		assert.Contains(t, row.FilterValue, "test-bucket")
		assert.Contains(t, row.FilterValue, "us-central1")
		assert.Contains(t, row.FilterValue, "STANDARD")
	})

	t.Run("ID is bucket name", func(t *testing.T) {
		assert.Equal(t, "test-bucket", row.ID)
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

func TestBucketsViewSetContext(t *testing.T) {
	view := NewBucketsView("test-project")
	ctx := &context.ProgramContext{
		ScreenWidth:   100,
		ScreenHeight:  50,
		ContentWidth:  80,
		ContentHeight: 45,
	}

	view.SetContext(ctx)

	// Verify context is stored
	assert.Equal(t, ctx, view.ctx)
}
