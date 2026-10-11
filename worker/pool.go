package worker

import (
	"context"
	"sync"
	"time"

	"github.com/alexanderritik/mini-lambda/model"
	"github.com/rs/zerolog/log"
)

// runPool claims jobs with next and runs handle on up to concurrency of them
// at once. It polls every interval and immediately when a slot frees up. When
// ctx is cancelled it stops claiming and returns once in-flight jobs finish.
//
// Jobs get a context detached from ctx's cancellation so a shutdown doesn't
// abort their database writes half way through.
func runPool(
	ctx context.Context,
	concurrency int,
	interval time.Duration,
	next func(ctx context.Context) (*model.Job, error),
	handle func(ctx context.Context, job *model.Job),
) {
	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	wake := make(chan struct{}, 1)
	var wg sync.WaitGroup
	jobCtx := context.WithoutCancel(ctx)

	fill := func() {
		for ctx.Err() == nil {
			select {
			case sem <- struct{}{}:
			default:
				return // all slots busy
			}
			job, err := next(ctx)
			if err != nil || job == nil {
				<-sem
				if err != nil && ctx.Err() == nil {
					log.Error().Err(err).Msg("failed to dequeue job")
				}
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					<-sem
					select {
					case wake <- struct{}{}:
					default:
					}
				}()
				handle(jobCtx, job)
			}()
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-ticker.C:
			fill()
		case <-wake:
			fill()
		}
	}
}
