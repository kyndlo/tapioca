package control

import (
	"context"
	"sync"
)

type CancellationRegistry struct {
	mu   sync.Mutex
	jobs map[string]context.CancelFunc
}

func NewCancellationRegistry() *CancellationRegistry {
	return &CancellationRegistry{jobs: make(map[string]context.CancelFunc)}
}

func (r *CancellationRegistry) Register(parent context.Context, jobID string) (context.Context, *ProtocolError) {
	if jobID == "" {
		return parent, nil
	}
	ctx, cancel := context.WithCancel(parent)

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.jobs[jobID]; exists {
		cancel()
		return nil, &ProtocolError{
			Code:      "job_conflict",
			Message:   "a job with this ID is already running",
			Retryable: false,
			Details:   map[string]any{"job_id": jobID},
		}
	}
	r.jobs[jobID] = cancel
	return ctx, nil
}

func (r *CancellationRegistry) Cancel(jobID string) bool {
	r.mu.Lock()
	cancel, exists := r.jobs[jobID]
	r.mu.Unlock()
	if exists {
		cancel()
	}
	return exists
}

func (r *CancellationRegistry) Complete(jobID string) {
	if jobID == "" {
		return
	}
	r.mu.Lock()
	delete(r.jobs, jobID)
	r.mu.Unlock()
}

func (r *CancellationRegistry) Contains(jobID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, exists := r.jobs[jobID]
	return exists
}
