// Package retention prunes old runs so the database and object storage don't
// grow without bound.
package retention

import (
	"context"
	"strings"
	"time"

	"github.com/alexanderritik/byot/repository"
	"github.com/alexanderritik/byot/storage"
	"github.com/rs/zerolog/log"
)

type Policy struct {
	MaxRunsPerTest int           // keep at most this many runs per test; 0 = no limit
	MaxAge         time.Duration // delete runs and finished jobs older than this; 0 = no limit
}

func (p Policy) Enabled() bool { return p.MaxRunsPerTest > 0 || p.MaxAge > 0 }

const (
	batchSize  = 200
	maxBatches = 25 // per sweep, so one pass can't run unbounded
)

type Sweeper struct {
	repo    repository.RetentionRepository
	storage storage.Storage
	policy  Policy
	now     func() time.Time
}

func New(repo repository.RetentionRepository, store storage.Storage, policy Policy) *Sweeper {
	return &Sweeper{repo: repo, storage: store, policy: policy, now: time.Now}
}

// Run sweeps shortly after start and then every interval, until ctx ends.
func (s *Sweeper) Run(ctx context.Context, interval time.Duration) {
	if !s.policy.Enabled() {
		log.Info().Msg("retention disabled")
		return
	}
	log.Info().Int("max_runs_per_test", s.policy.MaxRunsPerTest).Dur("max_age", s.policy.MaxAge).Msg("retention enabled")

	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.Sweep(ctx)
			timer.Reset(interval)
		}
	}
}

// Sweep deletes expired runs (storage first, then the database rows, so a
// storage failure leaves the row to be retried rather than orphaning files)
// and finished jobs past the age limit. It returns the number of runs removed.
func (s *Sweeper) Sweep(ctx context.Context) int {
	var cutoff *time.Time
	if s.policy.MaxAge > 0 {
		c := s.now().Add(-s.policy.MaxAge)
		cutoff = &c
	}

	total := 0
	for i := 0; i < maxBatches && ctx.Err() == nil; i++ {
		expired, err := s.repo.ListExpiredRuns(ctx, s.policy.MaxRunsPerTest, cutoff, batchSize)
		if err != nil {
			log.Error().Err(err).Msg("retention: list expired runs")
			break
		}
		if len(expired) == 0 {
			break
		}

		var ok []string
		for _, e := range expired {
			if err := s.deleteStorage(e); err != nil {
				log.Error().Err(err).Str("run_id", e.RunID).Msg("retention: delete run files; will retry")
				continue
			}
			ok = append(ok, e.RunID)
		}
		if len(ok) == 0 {
			break // nothing progressed; avoid spinning on the same failures
		}
		if err := s.repo.DeleteRuns(ctx, ok); err != nil {
			log.Error().Err(err).Msg("retention: delete run rows")
			break
		}
		total += len(ok)
	}

	jobs := int64(0)
	if cutoff != nil && ctx.Err() == nil {
		n, err := s.repo.DeleteFinishedJobsBefore(ctx, *cutoff)
		if err != nil {
			log.Error().Err(err).Msg("retention: delete old jobs")
		}
		jobs = n
	}

	if total > 0 || jobs > 0 {
		log.Info().Int("runs", total).Int64("jobs", jobs).Msg("retention sweep complete")
	}
	return total
}

// deleteStorage removes everything stored for one run: its log and screenshots
// live under <test>/log/<run>/. Older runs may have a log outside that folder.
func (s *Sweeper) deleteStorage(e repository.ExpiredRun) error {
	prefix := e.TestID + "/log/" + e.RunID + "/"
	if err := s.storage.DeletePrefix(prefix); err != nil {
		return err
	}
	if e.LogURL != "" && !strings.HasPrefix(e.LogURL, prefix) {
		return s.storage.DeleteObject(e.LogURL)
	}
	return nil
}
