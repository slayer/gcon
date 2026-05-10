package usage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/slayer/gcon/internal/gcp"
)

// errFakeMonitoring is a static sentinel error used by the monitoring-error test.
var errFakeMonitoring = errors.New("boom")

// fakeMonitoring is a programmable MonitoringFetcher.
type fakeMonitoring struct {
	mu    sync.Mutex
	calls int
	bytes int64
	count int64
	asOf  time.Time
	err   error
}

func (f *fakeMonitoring) FetchBucketUsage(_ context.Context, _ string) (bytes, count int64, asOf time.Time, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.bytes, f.count, f.asOf, f.err
}

// fakeStorage is a programmable StorageLister.
type fakeStorage struct {
	mu       sync.Mutex
	calls    int
	objs     []gcp.StorageObject
	err      error
	delay    time.Duration                       // optional pause to simulate a slow scan
	classes  func(o gcp.StorageObject) string    //nolint:unused // reserved for future per-object class injection
	listFunc func(ctx context.Context) error     // optional override for ListAllObjects
}

func (f *fakeStorage) ListAllObjects(ctx context.Context, _, _ string) ([]gcp.StorageObject, error) {
	f.mu.Lock()
	f.calls++
	listFunc := f.listFunc
	f.mu.Unlock()
	if listFunc != nil {
		if err := listFunc(ctx); err != nil {
			return nil, err
		}
		return f.objs, f.err
	}
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return f.objs, f.err
}

func TestScanner_GetCacheMiss(t *testing.T) {
	s := New(&fakeStorage{}, &fakeMonitoring{})
	_, ok := s.Get("any", "")
	assert.False(t, ok)
}

func TestScanner_FetchMonitoring_Success(t *testing.T) {
	mon := &fakeMonitoring{bytes: 999, count: 7, asOf: time.Now()}
	s := New(&fakeStorage{}, mon)
	cmd := s.FetchMonitoring(context.Background(), "b")
	require.NotNil(t, cmd)

	msg := cmd()
	ready, ok := msg.(ReadyMsg)
	require.True(t, ok, "expected ReadyMsg, got %T", msg)
	require.NoError(t, ready.Err)
	assert.Equal(t, int64(999), ready.Usage.TotalBytes)
	assert.Equal(t, int64(7), ready.Usage.ObjectCount)
	assert.Equal(t, SourceMonitoring, ready.Usage.Source)
	assert.Equal(t, "b", ready.Usage.Bucket)

	// Cache populated.
	cached, ok := s.Get("b", "")
	assert.True(t, ok)
	assert.Equal(t, int64(999), cached.TotalBytes)
}

func TestScanner_FetchMonitoring_CacheHit(t *testing.T) {
	mon := &fakeMonitoring{bytes: 10, asOf: time.Now()}
	s := New(&fakeStorage{}, mon)

	// Prime cache.
	s.FetchMonitoring(context.Background(), "b")()
	assert.Equal(t, 1, mon.calls)

	// Second call within TTL should be a no-op (cache hit).
	cmd := s.FetchMonitoring(context.Background(), "b")
	msg := cmd()
	if msg != nil {
		// If a message did come back, it must be a ReadyMsg from cache (no extra fetch).
		if _, ok := msg.(ReadyMsg); !ok {
			t.Fatalf("unexpected message type %T", msg)
		}
	}
	assert.Equal(t, 1, mon.calls, "cache hit should not call monitoring again")
}

// TestScanner_FetchMonitoring_CacheHitWithStaleAsOf is a regression test for
// the bug where ScannedAt was overwritten with asOf (the metric publication
// time, often many hours old). That defeated the 10-minute TTL because
// time.Since(asOf) > monitoringTTL on every check, so every nav re-hit
// Cloud Monitoring. The fix keeps ScannedAt = fetch time and stores asOf in
// a separate field. With the bug, this test fails with mon.calls == 2.
func TestScanner_FetchMonitoring_CacheHitWithStaleAsOf(t *testing.T) {
	mon := &fakeMonitoring{
		bytes: 10,
		// Simulate a metric whose publication time is 12 hours ago — typical
		// for storage.googleapis.com/storage/total_bytes which updates ~daily.
		asOf: time.Now().Add(-12 * time.Hour),
	}
	s := New(&fakeStorage{}, mon)

	// First call — populates the cache.
	first, ok := s.FetchMonitoring(context.Background(), "b")().(ReadyMsg)
	require.True(t, ok)
	require.NoError(t, first.Err)
	assert.Equal(t, 1, mon.calls)
	// AsOf surfaced for UI display, ScannedAt set to fetch-time (now).
	assert.False(t, first.Usage.AsOf.IsZero())
	assert.WithinDuration(t, time.Now(), first.Usage.ScannedAt, time.Second)

	// Second call — should be a cache hit (TTL not yet expired).
	second := s.FetchMonitoring(context.Background(), "b")()
	if second != nil {
		if _, ok := second.(ReadyMsg); !ok {
			t.Fatalf("unexpected message type %T", second)
		}
	}
	assert.Equal(t, 1, mon.calls, "stale-asOf cache hit must NOT trigger a second monitoring fetch")
}

func TestScanner_FetchMonitoring_Error(t *testing.T) {
	mon := &fakeMonitoring{err: errFakeMonitoring}
	s := New(&fakeStorage{}, mon)
	msg := s.FetchMonitoring(context.Background(), "b")()
	ready, ok := msg.(ReadyMsg)
	require.True(t, ok)
	assert.Error(t, ready.Err)
}

func TestScanner_Invalidate(t *testing.T) {
	mon := &fakeMonitoring{bytes: 1, asOf: time.Now()}
	s := New(&fakeStorage{}, mon)
	s.FetchMonitoring(context.Background(), "b")()
	require.Equal(t, 1, mon.calls)

	s.Invalidate("b", "")
	s.FetchMonitoring(context.Background(), "b")()
	assert.Equal(t, 2, mon.calls, "after invalidate, fetch should hit monitoring again")
}

func TestScanner_StartDeepScan_Success(t *testing.T) {
	st := &fakeStorage{
		objs: []gcp.StorageObject{
			{Name: "a.txt", Size: 100},
			{Name: "logs/b.log", Size: 200},
		},
	}
	s := New(st, &fakeMonitoring{})
	jobID, cmd := s.StartDeepScan(context.Background(), "b", "")
	require.NotEmpty(t, jobID)
	require.NotNil(t, cmd)

	// Pump until ReadyMsg.
	var ready ReadyMsg
	for {
		msg := cmd()
		if msg == nil {
			t.Fatal("nil message before ReadyMsg")
		}
		if r, ok := msg.(ReadyMsg); ok {
			ready = r
			break
		}
		// Otherwise it's a ProgressMsg — re-arm.
		cmd = s.NextMessage(jobID)
		require.NotNil(t, cmd)
	}
	require.NoError(t, ready.Err)
	assert.Equal(t, int64(300), ready.Usage.TotalBytes)
	assert.Equal(t, int64(2), ready.Usage.ObjectCount)
	assert.Equal(t, SourceDeepScan, ready.Usage.Source)

	// Cached for next caller.
	cached, ok := s.Get("b", "")
	assert.True(t, ok)
	assert.Equal(t, int64(300), cached.TotalBytes)
}

func TestScanner_StartDeepScan_Cancel(t *testing.T) {
	st := &fakeStorage{
		delay: 5 * time.Second,
		objs:  []gcp.StorageObject{{Name: "a", Size: 1}},
	}
	s := New(st, &fakeMonitoring{})
	jobID, cmd := s.StartDeepScan(context.Background(), "b", "")
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.Cancel(jobID)
	}()
	msg := cmd()
	ready, ok := msg.(ReadyMsg)
	require.True(t, ok, "expected ReadyMsg after cancel, got %T", msg)
	require.Error(t, ready.Err)
	assert.ErrorIs(t, ready.Err, context.Canceled)
}

// TestScanner_StartDeepScan_LateCancel exercises the cancel race where Cancel()
// arrives AFTER ListAllObjects has begun but the runner has not yet finished
// processing the result. The late cancellation must produce a ReadyMsg with
// context.Canceled, not a successful ReadyMsg.
func TestScanner_StartDeepScan_LateCancel(t *testing.T) {
	releaseList := make(chan struct{})
	st := &fakeStorage{
		objs: []gcp.StorageObject{{Name: "a", Size: 1}},
		listFunc: func(ctx context.Context) error {
			// Wait for the test to release; meanwhile the test will cancel.
			select {
			case <-releaseList:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
	s := New(st, &fakeMonitoring{})
	jobID, cmd := s.StartDeepScan(context.Background(), "b", "")

	// Cancel first, then release ListAllObjects — Cancel cancels the runner's
	// context. ListAllObjects will return ctx.Err() because of the inner select.
	s.Cancel(jobID)
	close(releaseList)

	msg := cmd()
	ready, ok := msg.(ReadyMsg)
	require.True(t, ok)
	require.Error(t, ready.Err)
	assert.ErrorIs(t, ready.Err, context.Canceled)
}

func TestScanner_StartDeepScan_Dedup(t *testing.T) {
	st := &fakeStorage{
		delay: 100 * time.Millisecond,
		objs:  []gcp.StorageObject{{Name: "a", Size: 1}},
	}
	s := New(st, &fakeMonitoring{})

	job1, cmd1 := s.StartDeepScan(context.Background(), "b", "")
	job2, cmd2 := s.StartDeepScan(context.Background(), "b", "")
	assert.Equal(t, job1, job2, "second call for same key should reuse jobID")
	require.NotNil(t, cmd1)
	require.NotNil(t, cmd2)

	// Both subscribers should receive the final ReadyMsg.
	got1 := drainToReady(t, s, job1, cmd1)
	got2 := drainToReady(t, s, job2, cmd2)
	require.NoError(t, got1.Err)
	require.NoError(t, got2.Err)
	assert.Equal(t, int64(1), got1.Usage.TotalBytes)
	assert.Equal(t, int64(1), got2.Usage.TotalBytes)
	assert.Equal(t, 1, st.calls, "only one underlying List call expected")
}

// closableMonitoring is a fakeMonitoring variant that also implements io.Closer
// so we can verify Scanner.Close() releases the monitoring client.
type closableMonitoring struct {
	fakeMonitoring
	closed bool
}

func (c *closableMonitoring) Close() error {
	c.closed = true
	return nil
}

func TestScanner_Close_ReleasesMonitoringClient(t *testing.T) {
	mon := &closableMonitoring{}
	s := New(&fakeStorage{}, mon)
	require.NoError(t, s.Close())
	assert.True(t, mon.closed, "Scanner.Close should propagate to monitoring closer")
}

func TestScanner_Close_CancelsInflightScans(t *testing.T) {
	st := &fakeStorage{
		delay: 5 * time.Second,
		objs:  []gcp.StorageObject{{Name: "a", Size: 1}},
	}
	s := New(st, &fakeMonitoring{})
	jobID, cmd := s.StartDeepScan(context.Background(), "b", "")

	// Close should cancel the running scan and produce ReadyMsg{Err: Canceled}.
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = s.Close() //nolint:errcheck // Close error not relevant to this test
	}()

	msg := cmd()
	ready, ok := msg.(ReadyMsg)
	require.True(t, ok, "expected ReadyMsg after Close, got %T", msg)
	assert.Equal(t, jobID, ready.JobID)
	assert.ErrorIs(t, ready.Err, context.Canceled)
}

func TestScanner_Close_Idempotent(t *testing.T) {
	s := New(&fakeStorage{}, &closableMonitoring{})
	require.NoError(t, s.Close())
	require.NoError(t, s.Close(), "second Close should be a no-op, not panic")
}

func drainToReady(t *testing.T, s *Scanner, jobID string, cmd tea.Cmd) ReadyMsg {
	t.Helper()
	for range 1000 {
		msg := cmd()
		if msg == nil {
			t.Fatal("nil message")
		}
		if r, ok := msg.(ReadyMsg); ok {
			return r
		}
		cmd = s.NextMessage(jobID)
		require.NotNil(t, cmd)
	}
	t.Fatal("never reached ReadyMsg")
	return ReadyMsg{}
}
