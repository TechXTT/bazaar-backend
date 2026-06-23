package algolia

import (
	"log"
	"time"
)

// BE-19: Algolia index mutations were previously fired as unbounded bare
// goroutines with dropped errors (one goroutine per write, no backpressure, no
// retry). Replace that with a bounded worker pool draining a buffered queue,
// retrying transient failures with backoff. If the queue is full we drop the
// job and log rather than spawning unbounded goroutines.

const (
	queueCapacity = 1024
	queueWorkers  = 4
	maxAttempts   = 3
	retryBackoff  = 500 * time.Millisecond
)

type jobKind int

const (
	jobIndex jobKind = iota
	jobDelete
)

type indexJob struct {
	kind   jobKind
	record ProductRecord
	id     string
}

// queue is a bounded, retrying async dispatcher for index mutations.
type queue struct {
	jobs chan indexJob
	svc  AlgoliaService
}

func newQueue(svc AlgoliaService) *queue {
	q := &queue{
		jobs: make(chan indexJob, queueCapacity),
		svc:  svc,
	}
	for i := 0; i < queueWorkers; i++ {
		go q.worker()
	}
	return q
}

func (q *queue) worker() {
	for job := range q.jobs {
		q.process(job)
	}
}

func (q *queue) process(job indexJob) {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		var err error
		switch job.kind {
		case jobIndex:
			err = q.svc.IndexProduct(job.record)
		case jobDelete:
			err = q.svc.DeleteProduct(job.id)
		}
		if err == nil {
			return
		}
		if attempt < maxAttempts {
			time.Sleep(retryBackoff * time.Duration(attempt))
		} else {
			log.Printf("algolia queue: giving up after %d attempts (kind=%d): %v", maxAttempts, job.kind, err)
		}
	}
}

func (q *queue) enqueue(job indexJob) {
	select {
	case q.jobs <- job:
	default:
		log.Printf("algolia queue: full (cap=%d), dropping job kind=%d", queueCapacity, job.kind)
	}
}
