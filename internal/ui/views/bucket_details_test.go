package views

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/gcp/usage"
	"github.com/slayer/gcon/internal/ui/context"
)

func TestBucketDetailsView_InitFiresMonitoringRequest(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	cmd := v.Init()
	require.NotNil(t, cmd)
	// Init returns a Batch that includes spinner.Tick + a closure emitting
	// UsageMonitoringRequestMsg. Don't introspect the batch; just ensure
	// Init() doesn't panic and produces something to dispatch.
}

func TestBucketDetailsView_DeepScanKeyEmitsRequest(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	// Switch to Usage tab so the 'C' key is honored.
	v.tabs.SetActive(1)

	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	require.NotNil(t, cmd, "Pressing 'C' on the Usage tab should emit a deep-scan request")

	got := cmd()
	req, ok := got.(UsageDeepScanRequestMsg)
	require.True(t, ok, "expected UsageDeepScanRequestMsg, got %T", got)
	assert.Equal(t, "b1", req.Bucket)
	assert.Equal(t, "", req.Prefix)
	assert.True(t, v.scanInProgress, "scanInProgress should flip to true after deep-scan request")
}

func TestBucketDetailsView_DeepScanIgnoredOnDetailsTab(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	// Stay on Details tab (index 0) — 'C' should be a no-op.
	cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	assert.Nil(t, cmd, "'C' on the Details tab should not trigger a scan")
	assert.False(t, v.scanInProgress)
}

func TestBucketDetailsView_HandlesReadyMsg(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	v.Update(usage.ReadyMsg{
		Usage: usage.BucketUsage{
			Bucket:     "b1",
			TotalBytes: 12345,
			Source:     usage.SourceMonitoring,
			ScannedAt:  time.Now(),
		},
	})
	require.NotNil(t, v.usage)
	assert.Equal(t, int64(12345), v.usage.TotalBytes)
	assert.False(t, v.scanInProgress)
}

func TestBucketDetailsView_IgnoresReadyMsgForOtherBucket(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	v.Update(usage.ReadyMsg{
		Usage: usage.BucketUsage{
			Bucket:     "different",
			TotalBytes: 999,
			Source:     usage.SourceMonitoring,
		},
	})
	assert.Nil(t, v.usage, "ReadyMsg for an unrelated bucket should be ignored")
}

func TestBucketDetailsView_ProgressMsgUpdatesRunningTally(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	v.Update(usage.ProgressMsg{
		Bucket:         "b1",
		ObjectsScanned: 100,
		BytesScanned:   1024,
	})
	require.NotNil(t, v.usage)
	assert.True(t, v.scanInProgress)
	assert.Equal(t, int64(1024), v.usage.TotalBytes)
	assert.Equal(t, int64(100), v.usage.ObjectCount)
	assert.Equal(t, usage.SourceDeepScan, v.usage.Source)
}

func TestBucketDetailsView_TabSwitching(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	assert.Equal(t, "details", v.tabs.ActiveTab().ID)

	// 'l' (NextTab)
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	assert.Equal(t, "usage", v.tabs.ActiveTab().ID)

	// 'h' (PrevTab)
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	assert.Equal(t, "details", v.tabs.ActiveTab().ID)

	// '2' jumps to second tab
	v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	assert.Equal(t, "usage", v.tabs.ActiveTab().ID)
}

func TestBucketDetailsView_ViewWithoutContextRendersEmpty(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	assert.Equal(t, "", v.View(), "View() returns empty when no context is set")
}

func TestBucketDetailsView_ViewRendersDetailsTab(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{
		Name:         "my-bucket",
		Location:     "US",
		StorageClass: "STANDARD",
	})
	ctx := context.New()
	ctx.SetDimensions(120, 40, 100, 35)
	v.SetContext(ctx)

	out := v.View()
	assert.Contains(t, out, "my-bucket")
	assert.Contains(t, out, "US")
	assert.Contains(t, out, "STANDARD")
}

func TestBucketDetailsView_ViewRendersUsageTabWithMonitoring(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "my-bucket"})
	ctx := context.New()
	ctx.SetDimensions(120, 40, 100, 35)
	v.SetContext(ctx)
	v.tabs.SetActive(1)
	v.Update(usage.ReadyMsg{
		Usage: usage.BucketUsage{
			Bucket:      "my-bucket",
			TotalBytes:  1024 * 1024,
			ObjectCount: 42,
			Source:      usage.SourceMonitoring,
			ScannedAt:   time.Now(),
		},
	})

	out := v.View()
	assert.Contains(t, out, "Total size:")
	assert.Contains(t, out, "Object count:")
	assert.Contains(t, out, "Monitoring")
}
