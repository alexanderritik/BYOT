package schedule

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// NextRun returns the next fire time in UTC after from (exclusive of past due handling).
func NextRun(cronExpr string, timezone string, from time.Time) (time.Time, error) {
	if cronExpr == "" {
		return time.Time{}, fmt.Errorf("empty cron expression")
	}

	sched, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cron: %w", err)
	}

	loc := time.UTC
	if timezone != "" {
		loaded, err := time.LoadLocation(timezone)
		if err != nil {
			return time.Time{}, fmt.Errorf("load timezone: %w", err)
		}
		loc = loaded
	}

	next := sched.Next(from.In(loc))
	return next.UTC(), nil
}
