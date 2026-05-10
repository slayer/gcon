package usage

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// monitoringTTL is how long a SourceMonitoring cache entry is considered fresh
// before the scanner will re-fetch from Cloud Monitoring.
const monitoringTTL = 10 * time.Minute

// Scanner is the central registry for bucket usage queries. It is safe for
// concurrent use; views and the App share a single instance per project.
type Scanner struct {
	storage    StorageLister
	monitoring MonitoringFetcher

	mu       sync.RWMutex
	cache    map[string]BucketUsage
	inflight map[string]*scanJob
}

// New constructs a Scanner with the given backends. The two arguments are
// interfaces so tests can supply fakes; production callers pass in
// *gcp.StorageClient and *gcp.MonitoringClient.
func New(storage StorageLister, monitoring MonitoringFetcher) *Scanner {
	return &Scanner{
		storage:    storage,
		monitoring: monitoring,
		cache:      make(map[string]BucketUsage),
		inflight:   make(map[string]*scanJob),
	}
}

// cacheKey is the stable in-memory key for a (bucket, prefix) pair.
func cacheKey(bucket, prefix string) string {
	return bucket + "|" + prefix
}

// Get returns the cached usage for (bucket, prefix), if any.
func (s *Scanner) Get(bucket, prefix string) (BucketUsage, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.cache[cacheKey(bucket, prefix)]
	return u, ok
}

// Invalidate drops the cached entry (if any) for (bucket, prefix). Does not
// cancel any in-flight scan for that key.
func (s *Scanner) Invalidate(bucket, prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, cacheKey(bucket, prefix))
}

// FetchMonitoring returns a tea.Cmd that fetches bucket-level totals from
// Cloud Monitoring and emits a ReadyMsg. If a fresh monitoring entry already
// exists in the cache, the returned Cmd emits that cached value immediately
// without contacting GCP.
//
// Monitoring entries are always keyed with prefix == "".
func (s *Scanner) FetchMonitoring(ctx context.Context, bucket string) tea.Cmd {
	key := cacheKey(bucket, "")

	// Cache check.
	s.mu.RLock()
	if u, ok := s.cache[key]; ok && u.Source == SourceMonitoring && time.Since(u.ScannedAt) < monitoringTTL {
		s.mu.RUnlock()
		cached := u
		return func() tea.Msg {
			return ReadyMsg{JobID: "monitoring:" + bucket, Usage: cached}
		}
	}
	s.mu.RUnlock()

	return func() tea.Msg {
		bytes, count, asOf, err := s.monitoring.FetchBucketUsage(ctx, bucket)
		if err != nil {
			return ReadyMsg{JobID: "monitoring:" + bucket, Err: err}
		}
		// Use ScannedAt as "when this record was created", not the asOf of the
		// underlying metric. asOf is preserved on the BucketUsage so the UI can
		// render "12h ago" — but cache TTL is based on when we fetched.
		u := BucketUsage{
			Bucket:      bucket,
			Prefix:      "",
			TotalBytes:  bytes,
			ObjectCount: count,
			Source:      SourceMonitoring,
			ScannedAt:   time.Now(),
		}
		// Repurpose ScannedAt only when asOf is meaningful, so the UI's
		// "X ago" hint reflects metric freshness rather than fetch freshness.
		if !asOf.IsZero() {
			u.ScannedAt = asOf
		}
		s.mu.Lock()
		s.cache[key] = u
		s.mu.Unlock()
		return ReadyMsg{JobID: "monitoring:" + bucket, Usage: u}
	}
}

// StartDeepScan begins (or joins) a deep scan for (bucket, prefix). It returns
// the job ID and a tea.Cmd that reads exactly one message from this caller's
// subscriber channel. The App handler MUST call Scanner.NextMessage(jobID) on
// every ProgressMsg to keep the pump alive. ReadyMsg ends the pump (channel
// closes; subsequent NextMessage calls are no-ops).
//
// If a scan for the same key is already in flight, returns the existing jobID
// and a fresh subscriber Cmd attached to the existing job.
func (s *Scanner) StartDeepScan(ctx context.Context, bucket, prefix string) (string, tea.Cmd) {
	key := cacheKey(bucket, prefix)
	s.mu.Lock()
	if existing, ok := s.inflight[key]; ok {
		ch := existing.addSubscriber()
		s.mu.Unlock()
		return existing.id, pumpCmd(ch)
	}
	scanCtx, cancel := context.WithCancel(ctx)
	job := newScanJob(jobIDFor(bucket, prefix), bucket, prefix, cancel)
	s.inflight[key] = job
	ch := job.addSubscriber()
	s.mu.Unlock()

	go s.runScan(scanCtx, job)

	return job.id, pumpCmd(ch)
}

// NextMessage returns a Cmd that reads the next message from the job's most
// recent subscriber channel. Used by the App handler to keep the pump alive
// across messages.
//
// Implementation: we look up the job and reuse its most recent subscriber
// channel. This assumes a single sequential pump per job, which is the
// standard pattern (App.Update is the sole consumer per scan). New
// subscribers are only added by StartDeepScan when a different caller joins
// an in-flight scan.
//
// Returns a no-op Cmd if the job is no longer in flight (completed, canceled,
// or never started).
func (s *Scanner) NextMessage(jobID string) tea.Cmd {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, job := range s.inflight {
		if job.id != jobID {
			continue
		}
		job.mu.Lock()
		ch := job.lastSubscriberLocked()
		job.mu.Unlock()
		if ch == nil {
			return noOpCmd()
		}
		// Reuse the most recently added subscriber channel.
		return pumpCmd(ch)
	}
	return noOpCmd()
}

// Cancel terminates the named in-flight job, causing the runner to exit and
// broadcast ReadyMsg{Err: context.Canceled}. No-op if jobID is unknown.
func (s *Scanner) Cancel(jobID string) {
	s.mu.RLock()
	var target *scanJob
	for _, job := range s.inflight {
		if job.id == jobID {
			target = job
			break
		}
	}
	s.mu.RUnlock()
	if target != nil {
		target.cancel()
	}
}
