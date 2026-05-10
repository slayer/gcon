package views

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
	"github.com/slayer/gcon/internal/gcp/usage"
	"github.com/slayer/gcon/internal/ui/context"
)

var (
	errBucketDetailsForeign       = errors.New("foreign")
	errBucketDetailsOurs          = errors.New("ours")
	errBucketDetailsMonitoring    = errors.New("monitoring API not enabled")
	errBucketDetailsTransient     = errors.New("transient")
	errBucketDetailsForeignBucket = errors.New("foreign bucket")
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
	assert.True(t, v.scanInProgress, "scanInProgress should flip to true after deep-scan request")

	// 'C' returns a tea.Batch(spinner.Tick, deep-scan request closure). Drain
	// the batch and verify one of the produced messages is the request.
	req := findDeepScanRequest(t, cmd)
	assert.Equal(t, "b1", req.Bucket)
	assert.Equal(t, "", req.Prefix)
}

// findDeepScanRequest invokes the cmd (which may be a Batch) and returns the
// UsageDeepScanRequestMsg it produces, failing the test if absent.
func findDeepScanRequest(t *testing.T, cmd tea.Cmd) UsageDeepScanRequestMsg {
	t.Helper()
	msg := cmd()
	switch m := msg.(type) {
	case UsageDeepScanRequestMsg:
		return m
	case tea.BatchMsg:
		for _, sub := range m {
			if sub == nil {
				continue
			}
			if r, ok := sub().(UsageDeepScanRequestMsg); ok {
				return r
			}
		}
	}
	t.Fatalf("did not find UsageDeepScanRequestMsg in cmd output (got %T)", msg)
	return UsageDeepScanRequestMsg{}
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

func TestBucketDetailsView_IgnoresFolderScopedReadyMsg(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	v.Update(usage.ReadyMsg{
		Usage: usage.BucketUsage{
			Bucket:     "b1",
			Prefix:     "logs/",
			TotalBytes: 999,
			Source:     usage.SourceDeepScan,
		},
	})
	assert.Nil(t, v.usage, "folder-scoped ReadyMsg should not populate bucket-level usage")
}

func TestBucketDetailsView_IgnoresFolderScopedProgress(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	v.Update(usage.ProgressMsg{Bucket: "b1", Prefix: "logs/", BytesScanned: 500})
	assert.Nil(t, v.usage, "folder-scoped ProgressMsg should not populate bucket-level usage")
}

func TestBucketDetailsView_ErrorOnlyAttributedWhenScanning(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	// Foreign JobID and not scanning: ignored.
	v.Update(usage.ReadyMsg{JobID: "scan:other|", Err: errBucketDetailsForeign})
	assert.Nil(t, v.scanErr, "foreign error must not be attributed when not scanning")
	// Our scan starts, our JobID lands: error is attributed.
	v.scanInProgress = true
	v.Update(usage.ReadyMsg{JobID: "scan:b1|", Err: errBucketDetailsOurs})
	assert.NotNil(t, v.scanErr)
	assert.False(t, v.scanInProgress)
}

// TestBucketDetailsView_ForeignErrorIgnoredEvenWhenScanInProgress verifies
// that errors from concurrent scans (different bucket, or folder-scoped scan
// from ObjectsView in our bucket) are not mis-attributed to this view, even
// when this view has its own scan in flight. Gating is by JobID, not by the
// scanInProgress flag alone.
func TestBucketDetailsView_ForeignErrorIgnoredEvenWhenScanInProgress(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	v.scanInProgress = true
	// Foreign scan error (different bucket).
	v.Update(usage.ReadyMsg{JobID: "scan:other-bucket|", Err: errBucketDetailsForeign})
	assert.Nil(t, v.scanErr, "foreign-bucket error must not be attributed even when our scan is in progress")
	assert.True(t, v.scanInProgress, "our scan-in-progress flag must not be cleared by foreign error")
	// Folder-scoped error from our own bucket (same bucket, non-empty prefix).
	v.Update(usage.ReadyMsg{JobID: "scan:b1|logs/", Err: errBucketDetailsForeign})
	assert.Nil(t, v.scanErr, "folder-scoped error from our bucket must not be attributed")
	assert.True(t, v.scanInProgress)
	// Our error (matching JobID with empty prefix).
	v.Update(usage.ReadyMsg{JobID: "scan:b1|", Err: errBucketDetailsOurs})
	assert.NotNil(t, v.scanErr)
	assert.False(t, v.scanInProgress)
}

func TestBucketDetailsView_TabKeyCyclesTabs(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	require.Equal(t, "details", v.tabs.ActiveTab().ID)
	v.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "usage", v.tabs.ActiveTab().ID)
}

// TestBucketDetailsView_MonitoringErrorIsRendered verifies that a ReadyMsg
// with JobID "monitoring:<bucket>" and a non-nil Err is captured in
// monitoringErr (rather than silently dropped because the JobID doesn't match
// the deep-scan prefix), and that the Usage tab surfaces the error.
func TestBucketDetailsView_MonitoringErrorIsRendered(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	ctx := context.New()
	ctx.SetDimensions(120, 40, 100, 35)
	v.SetContext(ctx)
	v.tabs.SetActive(1) // Usage tab

	v.Update(usage.ReadyMsg{JobID: "monitoring:b1", Err: errBucketDetailsMonitoring})

	require.NotNil(t, v.monitoringErr)
	assert.Equal(t, errBucketDetailsMonitoring, v.monitoringErr)

	out := v.View()
	assert.Contains(t, out, "Monitoring error")
	assert.Contains(t, out, "monitoring API not enabled")
}

// TestBucketDetailsView_MonitoringErrorClearedOnSuccess verifies that a later
// successful monitoring fetch clears any prior monitoring error.
func TestBucketDetailsView_MonitoringErrorClearedOnSuccess(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	v.Update(usage.ReadyMsg{JobID: "monitoring:b1", Err: errBucketDetailsTransient})
	require.NotNil(t, v.monitoringErr)

	v.Update(usage.ReadyMsg{
		Usage: usage.BucketUsage{
			Bucket:     "b1",
			TotalBytes: 100,
			Source:     usage.SourceMonitoring,
			ScannedAt:  time.Now(),
		},
	})
	assert.Nil(t, v.monitoringErr, "successful fetch should clear the prior monitoring error")
}

// TestBucketDetailsView_MonitoringErrorForOtherBucketIgnored verifies the
// JobID gating works: monitoring errors for a *different* bucket must not
// be attributed to this view.
func TestBucketDetailsView_MonitoringErrorForOtherBucketIgnored(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	v.Update(usage.ReadyMsg{JobID: "monitoring:other", Err: errBucketDetailsForeignBucket})
	assert.Nil(t, v.monitoringErr, "monitoring error for a different bucket must not be attributed")
}

func TestBucketDetailsView_EnterEmitsBucketSelected(t *testing.T) {
	v := NewBucketDetailsView(gcp.Bucket{Name: "b1"})
	cmd := v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	sel, ok := msg.(BucketSelectedMsg)
	require.True(t, ok, "expected BucketSelectedMsg, got %T", msg)
	assert.Equal(t, "b1", sel.Bucket.Name)
}
