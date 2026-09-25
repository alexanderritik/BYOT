package schedule

import (
	"testing"
	"time"
)

func TestNextRun_UTC(t *testing.T) {
	from := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	next, err := NextRun("*/15 * * * *", from)
	if err != nil {
		t.Fatal(err)
	}
	if !next.After(from) {
		t.Fatalf("next %v should be after %v", next, from)
	}
	if next.Minute()%15 != 0 {
		t.Fatalf("expected 15-minute boundary, got %v", next)
	}
}
