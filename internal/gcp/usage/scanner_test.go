package usage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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
