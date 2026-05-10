package views

import (
	"testing"
	"time"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/gcp/usage"
	"github.com/slayer/gcon/internal/ui/components/table"
	"github.com/slayer/gcon/internal/ui/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		// Size and Objects (cols 3, 4) start as placeholder until usage arrives.
		assert.Equal(t, "…", row.Data[3])
		assert.Equal(t, "…", row.Data[4])
		assert.Equal(t, "2024-01-15", row.Data[5])
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

func TestBucketsView_UsageReadyUpdatesRow(t *testing.T) {
	v := NewBucketsView("p")
	// Skip the normal load — synthesize the loaded state directly.
	v.loading = false
	v.buckets = []gcp.Bucket{
		{Name: "b1", Location: "us", StorageClass: "STANDARD", Created: time.Now()},
		{Name: "b2", Location: "eu", StorageClass: "NEARLINE", Created: time.Now()},
	}
	rows := []table.Row{bucketToRow(v.buckets[0]), bucketToRow(v.buckets[1])}
	v.table.SetRows(rows)

	// Inject a ReadyMsg for b1.
	v.Update(usage.ReadyMsg{
		JobID: "monitoring:b1",
		Usage: usage.BucketUsage{
			Bucket:      "b1",
			TotalBytes:  1_500_000_000,
			ObjectCount: 4321,
			Source:      usage.SourceMonitoring,
			ScannedAt:   time.Now().Add(-2 * time.Hour),
		},
	})

	// Locate the b1 row and check Size and Objects cells.
	got := v.table.Rows()
	require.Len(t, got, 2)
	row := got[0]
	assert.Contains(t, row.Data[3], "1.4 GB", "Size cell should be human formatted (1.5e9 ~ 1.4 GiB)")
	// Object count should be present (formatting may vary - thousands grouped).
	assert.Contains(t, row.Data[4], "4,321")
}

func TestBucketsView_ProgressDoesNotShowVerifiedMark(t *testing.T) {
	v := NewBucketsView("p")
	v.loading = false
	v.buckets = []gcp.Bucket{{Name: "b1", Location: "us", StorageClass: "STANDARD", Created: time.Now()}}
	v.table.SetRows([]table.Row{bucketToRow(v.buckets[0])})

	// In-progress deep scan tick: should update numbers but NOT add ✓.
	v.Update(usage.ProgressMsg{
		JobID:          "scan:b1|",
		Bucket:         "b1",
		BytesScanned:   100,
		ObjectsScanned: 1,
	})
	row := v.table.Rows()[0]
	assert.NotContains(t, row.Data[3], "✓",
		"in-progress scan must not show the ✓ verified marker")

	// Final ReadyMsg: should add ✓ to deep-scan results.
	v.Update(usage.ReadyMsg{
		JobID: "scan:b1|",
		Usage: usage.BucketUsage{
			Bucket:      "b1",
			TotalBytes:  500,
			ObjectCount: 5,
			Source:      usage.SourceDeepScan,
			ScannedAt:   time.Now(),
		},
	})
	row = v.table.Rows()[0]
	assert.Contains(t, row.Data[3], "✓",
		"final deep-scan result must show the ✓ verified marker")
}

func TestBucketsView_LateProgressIgnoredAfterFinal(t *testing.T) {
	v := NewBucketsView("p")
	v.loading = false
	v.buckets = []gcp.Bucket{{Name: "b1", Location: "us", StorageClass: "STANDARD", Created: time.Now()}}
	v.table.SetRows([]table.Row{bucketToRow(v.buckets[0])})

	final := time.Now()
	// Final result lands first.
	v.Update(usage.ReadyMsg{
		JobID: "scan:b1|",
		Usage: usage.BucketUsage{
			Bucket:      "b1",
			TotalBytes:  500_000_000,
			ObjectCount: 5000,
			Source:      usage.SourceDeepScan,
			ScannedAt:   final,
		},
	})
	rowAfterFinal := v.table.Rows()[0].Data[3]

	// Stale ProgressMsg arrives later (channel reorder). The current ProgressMsg
	// path uses time.Now() as ScannedAt, which is AFTER `final` here. To simulate
	// a true reorder we instead drive applyUsage directly with an older timestamp.
	v.applyUsage(usage.BucketUsage{
		Bucket:      "b1",
		TotalBytes:  100,
		ObjectCount: 1,
		Source:      usage.SourceDeepScan,
		ScannedAt:   final.Add(-1 * time.Second),
	}, false)

	// Row should still display the final value.
	assert.Equal(t, rowAfterFinal, v.table.Rows()[0].Data[3],
		"late-arriving progress must not overwrite final deep-scan result")
}

func TestBucketsView_ReadyError_DoesNotOverwriteRow(t *testing.T) {
	v := NewBucketsView("p")
	v.loading = false
	v.buckets = []gcp.Bucket{{Name: "b1", Location: "us", StorageClass: "STANDARD", Created: time.Now()}}
	v.table.SetRows([]table.Row{bucketToRow(v.buckets[0])})

	// Seed a real value first.
	v.Update(usage.ReadyMsg{
		JobID: "monitoring:b1",
		Usage: usage.BucketUsage{
			Bucket: "b1", TotalBytes: 9_000_000, ObjectCount: 42,
			Source: usage.SourceMonitoring, ScannedAt: time.Now(),
		},
	})
	before := v.table.Rows()[0].Data[3]

	// Error ReadyMsg should NOT clear the cell to "0 B".
	v.Update(usage.ReadyMsg{JobID: "scan:b1|", Err: assert.AnError})
	after := v.table.Rows()[0].Data[3]
	assert.Equal(t, before, after, "error ReadyMsg must not overwrite displayed totals")
}

func TestBucketsView_SortPreservedAcrossUsageUpdate(t *testing.T) {
	v := NewBucketsView("p")
	v.loading = false
	v.buckets = []gcp.Bucket{
		{Name: "alpha", Location: "us", StorageClass: "STANDARD", Created: time.Now()},
		{Name: "beta", Location: "eu", StorageClass: "NEARLINE", Created: time.Now()},
	}
	v.table.SetRows([]table.Row{bucketToRow(v.buckets[0]), bucketToRow(v.buckets[1])})

	// User sorts by Location (col 1) descending.
	v.table.SortBy(1, false)
	col, asc := v.table.SortState()
	assert.Equal(t, 1, col)
	assert.False(t, asc)

	// Usage update arrives — applyUsage rebuilds rows. Sort must persist.
	v.Update(usage.ReadyMsg{
		JobID: "monitoring:alpha",
		Usage: usage.BucketUsage{
			Bucket:      "alpha",
			TotalBytes:  1024,
			ObjectCount: 10,
			Source:      usage.SourceMonitoring,
			ScannedAt:   time.Now(),
		},
	})
	col, asc = v.table.SortState()
	assert.Equal(t, 1, col, "applyUsage must preserve user's sort column")
	assert.False(t, asc, "applyUsage must preserve user's sort direction")
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
