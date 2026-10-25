package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexanderritik/byot/model"
)

func fakeQueue(n int) func(context.Context) (*model.Job, error) {
	var mu sync.Mutex
	i := 0
	return func(context.Context) (*model.Job, error) {
		mu.Lock()
		defer mu.Unlock()
		if i >= n {
			return nil, nil
		}
		i++
		return &model.Job{UUID: fmt.Sprint(i)}, nil
	}
}

func TestPoolRunsInParallelUpToLimit(t *testing.T) {
	var running, peak, done atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})

	go func() {
		runPool(ctx, 3, 10*time.Millisecond, fakeQueue(9), func(_ context.Context, _ *model.Job) {
			cur := running.Add(1)
			for {
				p := peak.Load()
				if cur <= p || peak.CompareAndSwap(p, cur) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			running.Add(-1)
			done.Add(1)
		})
		close(finished)
	}()

	deadline := time.After(3 * time.Second)
	for done.Load() < 9 {
		select {
		case <-deadline:
			t.Fatalf("only %d/9 jobs done", done.Load())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	<-finished

	if peak.Load() != 3 {
		t.Fatalf("peak concurrency = %d, want 3", peak.Load())
	}
}

func TestPoolDrainsInFlightOnShutdown(t *testing.T) {
	var finishedJob atomic.Bool
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})

	go func() {
		runPool(ctx, 2, 10*time.Millisecond, fakeQueue(1), func(jctx context.Context, _ *model.Job) {
			close(started)
			time.Sleep(100 * time.Millisecond)
			if jctx.Err() != nil { // job ctx must survive shutdown
				return
			}
			finishedJob.Store(true)
		})
		close(returned)
	}()

	<-started
	cancel()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("runPool did not return")
	}
	if !finishedJob.Load() {
		t.Fatal("in-flight job was not allowed to finish")
	}
}
