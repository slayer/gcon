package usage

import (
	"context"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/slayer/gcon/internal/gcp"
)

// scanJob represents one in-flight deep scan. Multiple subscribers (one per
// caller of StartDeepScan for the same key) each get their own buffered
// channel. The runner goroutine fans out every message to every subscriber.
type scanJob struct {
	id     string
	bucket string
	prefix string
	cancel context.CancelFunc

	mu          sync.Mutex
	subscribers []chan tea.Msg
	done        bool
}

// channelBuffer sizes each subscriber channel. Large enough to absorb a few
// progress messages plus the final ready message without blocking the runner.
const channelBuffer = 8

func newScanJob(id, bucket, prefix string, cancel context.CancelFunc) *scanJob {
	return &scanJob{
		id:     id,
		bucket: bucket,
		prefix: prefix,
		cancel: cancel,
	}
}

// addSubscriber returns a fresh channel that will receive every message broadcast
// to the job from now until completion.
func (j *scanJob) addSubscriber() chan tea.Msg {
	ch := make(chan tea.Msg, channelBuffer)
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.done {
		// Job already finished; close immediately so reader unblocks with nil.
		close(ch)
		return ch
	}
	j.subscribers = append(j.subscribers, ch)
	return ch
}

// broadcast sends msg to all subscribers (non-blocking; drops if buffer is full).
func (j *scanJob) broadcast(msg tea.Msg) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, ch := range j.subscribers {
		select {
		case ch <- msg:
		default:
			// Subscriber is slow; drop. Progress is approximate by design.
		}
	}
}

// closeAll closes every subscriber channel (called after the final ReadyMsg
// has been broadcast).
func (j *scanJob) closeAll() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.done = true
	for _, ch := range j.subscribers {
		close(ch)
	}
	j.subscribers = nil
}

// runScan executes the actual scan and broadcasts progress + ready messages.
// Caller must have already added the job to scanner.inflight under scanner.mu.
func (s *Scanner) runScan(ctx context.Context, job *scanJob) {
	objs, err := s.storage.ListAllObjects(ctx, job.bucket, job.prefix)

	// Always remove the job from inflight before the final broadcast so that
	// a subscriber re-subscribing after they see ReadyMsg gets a fresh job.
	s.mu.Lock()
	delete(s.inflight, cacheKey(job.bucket, job.prefix))
	s.mu.Unlock()

	if err != nil {
		job.broadcast(ReadyMsg{JobID: job.id, Err: err})
		job.closeAll()
		return
	}

	// Storage class isn't carried on gcp.StorageObject yet. For v1 we pass empty
	// strings (so ByStorageClass remains empty until that field is added). This
	// keeps tally semantics correct: zero entries instead of bogus ones.
	classes := make([]string, len(objs))
	usage := tallyObjects(job.bucket, job.prefix, objs, classes)

	// Cache the result.
	s.mu.Lock()
	s.cache[cacheKey(job.bucket, job.prefix)] = usage
	s.mu.Unlock()

	job.broadcast(ReadyMsg{JobID: job.id, Usage: usage})
	job.closeAll()
}

// pumpCmd returns a tea.Cmd that reads exactly one message from ch. When the
// channel is closed (job complete and drained), the Cmd returns nil — the App
// handler treats nil as "stop pumping".
func pumpCmd(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// jobIDFor returns the deterministic job ID used for a (bucket, prefix). Using
// a deterministic ID makes dedup trivial: same key → same ID → same job.
func jobIDFor(bucket, prefix string) string {
	return "scan:" + bucket + "|" + prefix
}

// _ unused-import guard to keep the gcp import even when fields aren't referenced.
var _ gcp.StorageObject
