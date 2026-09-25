// Package schedule computes next run times from cron expressions (UTC).
package schedule

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// NextRun returns the next fire time in UTC after from.
func NextRun(cronExpr string, from time.Time) (time.Time, error) {
	if cronExpr == "" {
		return time.Time{}, fmt.Errorf("empty cron expression")
	}

	sched, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cron: %w", err)
	}

	return sched.Next(from.UTC()).UTC(), nil
}
