package retention

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/alexanderritik/byot/repository"
)

type fakeRepo struct {
	runs    []repository.ExpiredRun // everything currently "expired"
	deleted []string
	jobsCut *time.Time
}

func (f *fakeRepo) ListExpiredRuns(_ context.Context, _ int, _ *time.Time, limit int) ([]repository.ExpiredRun, error) {
	if len(f.runs) > limit {
		return f.runs[:limit], nil
	}
	return f.runs, nil
}

func (f *fakeRepo) DeleteRuns(_ context.Context, ids []string) error {
	f.deleted = append(f.deleted, ids...)
	gone := map[string]bool{}
	for _, id := range ids {
		gone[id] = true
	}
	var keep []repository.ExpiredRun
	for _, r := range f.runs {
		if !gone[r.RunID] {
			keep = append(keep, r)
		}
	}
	f.runs = keep
	return nil
}

func (f *fakeRepo) DeleteFinishedJobsBefore(_ context.Context, c time.Time) (int64, error) {
	f.jobsCut = &c
	return 0, nil
}

type fakeStore struct {
	prefixes []string
	objects  []string
	failFor  string // prefix substring that errors
}

func (s *fakeStore) DeletePrefix(p string) error {
	if s.failFor != "" && contains(p, s.failFor) {
		return errors.New("boom")
	}
	s.prefixes = append(s.prefixes, p)
	return nil
}
func (s *fakeStore) DeleteObject(o string) error { s.objects = append(s.objects, o); return nil }
func (s *fakeStore) UploadArtifact(string, io.Reader, int64) (string, error) { return "", nil }
func (s *fakeStore) UploadLog(string, string, io.Reader, int64) (string, error) { return "", nil }
func (s *fakeStore) UploadBlob(string, io.Reader, int64) (string, error)    { return "", nil }
func (s *fakeStore) DownloadBlob(string) (io.Reader, error)                 { return nil, nil }

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestSweepDeletesStorageThenRows(t *testing.T) {
	repo := &fakeRepo{runs: []repository.ExpiredRun{
		{RunID: "r1", TestID: "t1", LogURL: "t1/log/r1/log"},
		{RunID: "r2", TestID: "t1", LogURL: "legacy/r2.log"}, // outside the run folder
	}}
	st := &fakeStore{}
	n := New(repo, st, Policy{MaxRunsPerTest: 5, MaxAge: time.Hour}).Sweep(context.Background())

	if n != 2 || len(repo.deleted) != 2 {
		t.Fatalf("deleted %d runs, want 2", n)
	}
	if st.prefixes[0] != "t1/log/r1/" || len(st.objects) != 1 || st.objects[0] != "legacy/r2.log" {
		t.Fatalf("storage calls: %v %v", st.prefixes, st.objects)
	}
	if repo.jobsCut == nil {
		t.Fatal("old jobs should be pruned when MaxAge is set")
	}
}

func TestSweepKeepsRowWhenStorageFails(t *testing.T) {
	repo := &fakeRepo{runs: []repository.ExpiredRun{
		{RunID: "bad", TestID: "t1"},
		{RunID: "good", TestID: "t2"},
	}}
	st := &fakeStore{failFor: "t1/"}
	done := make(chan int, 1)
	go func() { done <- New(repo, st, Policy{MaxRunsPerTest: 1}).Sweep(context.Background()) }()

	select {
	case n := <-done:
		if n != 1 || len(repo.deleted) != 1 || repo.deleted[0] != "good" {
			t.Fatalf("deleted=%v n=%d", repo.deleted, n)
		}
		if len(repo.runs) != 1 || repo.runs[0].RunID != "bad" {
			t.Fatal("run with failed storage delete must be kept for retry")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("sweep spun on a persistent failure")
	}
}

func TestPolicyDisabledAndNoJobPruneWithoutAge(t *testing.T) {
	if (Policy{}).Enabled() {
		t.Fatal("zero policy must be disabled")
	}
	repo := &fakeRepo{}
	New(repo, &fakeStore{}, Policy{MaxRunsPerTest: 3}).Sweep(context.Background())
	if repo.jobsCut != nil {
		t.Fatal("jobs must not be pruned when only a run cap is set")
	}
}
