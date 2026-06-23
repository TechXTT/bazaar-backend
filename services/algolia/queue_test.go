package algolia

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeService records calls and can fail a configurable number of times before
// succeeding, to exercise the queue's retry.
type fakeService struct {
	mu          sync.Mutex
	indexCalls  int
	deleteCalls int
	failTimes   int
	done        chan struct{}
}

func (f *fakeService) IndexProduct(p ProductRecord) error {
	f.mu.Lock()
	f.indexCalls++
	fail := f.indexCalls <= f.failTimes
	last := !fail
	f.mu.Unlock()
	if last && f.done != nil {
		close(f.done)
	}
	if fail {
		return errors.New("transient")
	}
	return nil
}

func (f *fakeService) DeleteProduct(id string) error {
	f.mu.Lock()
	f.deleteCalls++
	f.mu.Unlock()
	if f.done != nil {
		close(f.done)
	}
	return nil
}

func (f *fakeService) BulkIndex(_ []ProductRecord) error { return nil }
func (f *fakeService) EnqueueIndex(_ ProductRecord)      {}
func (f *fakeService) EnqueueDelete(_ string)            {}

// TestQueueRetriesUntilSuccess covers BE-19: a transient failure is retried.
func TestQueueRetriesUntilSuccess(t *testing.T) {
	f := &fakeService{failTimes: 2, done: make(chan struct{})}
	q := newQueue(f)

	q.enqueue(indexJob{kind: jobIndex, record: ProductRecord{ObjectID: "p1"}})

	select {
	case <-f.done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for index job to succeed")
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.indexCalls != 3 {
		t.Fatalf("expected 3 attempts (2 fail + 1 success), got %d", f.indexCalls)
	}
}

// TestQueueDropsWhenFull covers BE-19: a full queue drops jobs instead of
// growing unbounded.
func TestQueueDropsWhenFull(t *testing.T) {
	// A queue whose service blocks forever, so workers never drain.
	block := make(chan struct{})
	bs := &blockingService{block: block}
	q := newQueue(bs)

	// Fill the buffer plus the in-flight worker slots, then prove enqueue does
	// not panic or block once capacity is exceeded.
	for i := 0; i < queueCapacity+queueWorkers+10; i++ {
		q.enqueue(indexJob{kind: jobDelete, id: "x"})
	}
	close(block) // let workers exit
}

type blockingService struct{ block chan struct{} }

func (b *blockingService) IndexProduct(_ ProductRecord) error { <-b.block; return nil }
func (b *blockingService) DeleteProduct(_ string) error       { <-b.block; return nil }
func (b *blockingService) BulkIndex(_ []ProductRecord) error  { return nil }
func (b *blockingService) EnqueueIndex(_ ProductRecord)       {}
func (b *blockingService) EnqueueDelete(_ string)             {}
