package analytics

import (
	"context"
	"log"
	"sync"
	"time"
)

const (
	// DefaultFlushInterval is how often buffered visits are written to the database.
	DefaultFlushInterval = 5 * time.Minute
)

// Flusher periodically drains VisitCacheStore into VisitRepository.
type Flusher struct {
	cache    VisitCacheStore
	repo     VisitRepository
	interval time.Duration

	stopCh    chan struct{}
	doneCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewFlusher constructs a flush worker.
// Inputs: cache buffer, repository, interval (zero uses DefaultFlushInterval).
// Output: pointer to Flusher (not started).
func NewFlusher(cache VisitCacheStore, repo VisitRepository, interval time.Duration) *Flusher {
	if interval <= 0 {
		interval = DefaultFlushInterval
	}
	return &Flusher{
		cache:    cache,
		repo:     repo,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the background ticker loop exactly once.
// Inputs: none.
// Output: none (goroutine in the background).
func (f *Flusher) Start() {
	if f == nil {
		return
	}
	f.startOnce.Do(func() {
		f.doneCh = make(chan struct{})
		go f.loop()
	})
}

// Stop signals the loop to perform a final flush and waits until it exits.
// Inputs: none.
// Output: none.
func (f *Flusher) Stop() {
	if f == nil {
		return
	}
	f.stopOnce.Do(func() {
		close(f.stopCh)
	})
	if f.doneCh != nil {
		<-f.doneCh
	}
}

// FlushNow drains the cache and persists visits. On persist failure, visits are re-queued.
// Inputs: ctx forwarded to cache and repository.
// Output: persist error after re-queue attempt, if any.
func (f *Flusher) FlushNow(ctx context.Context) error {
	if f == nil || f.cache == nil {
		return nil
	}
	visits, err := f.cache.FlushAll(ctx)
	if err != nil {
		return err
	}
	if len(visits) == 0 {
		return nil
	}
	if f.repo == nil {
		return requeueVisits(ctx, f.cache, visits)
	}
	if err := f.repo.PersistVisits(ctx, visits); err != nil {
		if rqErr := requeueVisits(ctx, f.cache, visits); rqErr != nil {
			log.Printf("analytics: persist failed (%v) and requeue failed: %v", err, rqErr)
		}
		return err
	}
	return nil
}

// loop ticks every interval until Stop, then flushes once more.
// Inputs: none (uses Flusher fields).
// Output: none.
func (f *Flusher) loop() {
	defer close(f.doneCh)
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := f.FlushNow(context.Background()); err != nil {
				log.Printf("analytics: flush visits: %v", err)
			}
		case <-f.stopCh:
			if err := f.FlushNow(context.Background()); err != nil {
				log.Printf("analytics: final flush visits: %v", err)
			}
			return
		}
	}
}

// requeueVisits pushes visits back into the cache after a failed persist.
func requeueVisits(ctx context.Context, cache VisitCacheStore, visits []VisitInfo) error {
	if cache == nil {
		return nil
	}
	var first error
	for _, v := range visits {
		if err := cache.PushVisit(ctx, v); err != nil && first == nil {
			first = err
		}
	}
	return first
}
