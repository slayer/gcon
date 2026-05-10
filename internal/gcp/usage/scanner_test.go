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

// fakeMonitoring is a programmable MonitoringFetcher.
type fakeMonitoring struct {
	mu    sync.Mutex
	calls int
	bytes int64
	count int64
	asOf  time.Time
	err   error
}

func (f *fakeMonitoring) FetchBucketUsage(_ context.Context, _ string) (int64, int64, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.bytes, f.count, f.asOf, f.err
}

// fakeStorage is a programmable StorageLister.
type fakeStorage struct {
	mu      sync.Mutex
	calls   int
	objs    []gcp.StorageObject
	err     error
	delay   time.Duration // optional pause to simulate a slow scan
	classes func(o gcp.StorageObject) string
}

func (f *fakeStorage) ListAllObjects(ctx context.Context, _, _ string) ([]gcp.StorageObject, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
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

func TestScanner_FetchMonitoring_Error(t *testing.T) {
	mon := &fakeMonitoring{err: errors.New("boom")}
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

func drainToReady(t *testing.T, s *Scanner, jobID string, cmd tea.Cmd) ReadyMsg {
	t.Helper()
	for i := 0; i < 1000; i++ {
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
