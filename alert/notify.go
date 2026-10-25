package alert

import (
	"context"
	"errors"
	"fmt"
)

// Targets are the alert channels configured on a test. Empty fields are skipped.
type Targets struct {
	WebhookURL   string
	PagerDutyKey string
}

func (t Targets) Any() bool {
	return t.WebhookURL != "" || t.PagerDutyKey != ""
}

// NotifyFailure sends a failure alert to every configured channel. One
// channel failing doesn't stop the others; errors are joined.
func NotifyFailure(ctx context.Context, t Targets, p Payload) error {
	var errs []error
	if t.WebhookURL != "" {
		if err := SendFailure(ctx, t.WebhookURL, p); err != nil {
			errs = append(errs, fmt.Errorf("webhook: %w", err))
		}
	}
	if t.PagerDutyKey != "" {
		if err := TriggerPagerDuty(ctx, t.PagerDutyKey, p); err != nil {
			errs = append(errs, fmt.Errorf("pagerduty: %w", err))
		}
	}
	return errors.Join(errs...)
}

// NotifyRecovery sends a recovery notice and resolves the PagerDuty incident.
func NotifyRecovery(ctx context.Context, t Targets, p Payload) error {
	var errs []error
	if t.WebhookURL != "" {
		if err := SendRecovery(ctx, t.WebhookURL, p); err != nil {
			errs = append(errs, fmt.Errorf("webhook: %w", err))
		}
	}
	if t.PagerDutyKey != "" {
		if err := ResolvePagerDuty(ctx, t.PagerDutyKey, p); err != nil {
			errs = append(errs, fmt.Errorf("pagerduty: %w", err))
		}
	}
	return errors.Join(errs...)
}
