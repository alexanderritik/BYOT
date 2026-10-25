package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// PagerDuty Events API v2. Each test maps to one incident: the dedup key is
// the test ID, so repeat failure alerts land on the open incident and a
// recovery resolves it.
const defaultPagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

// pagerDutyEventsURL can be overridden with PAGERDUTY_EVENTS_URL, e.g.
// https://events.eu.pagerduty.com/v2/enqueue for EU service regions.
func pagerDutyEventsURL() string {
	if u := os.Getenv("PAGERDUTY_EVENTS_URL"); u != "" {
		return u
	}
	return defaultPagerDutyEventsURL
}

type pagerDutyEvent struct {
	RoutingKey  string            `json:"routing_key"`
	EventAction string            `json:"event_action"`
	DedupKey    string            `json:"dedup_key"`
	Payload     *pagerDutyPayload `json:"payload,omitempty"`
}

type pagerDutyPayload struct {
	Summary       string         `json:"summary"`
	Source        string         `json:"source"`
	Severity      string         `json:"severity"`
	Component     string         `json:"component,omitempty"`
	CustomDetails map[string]any `json:"custom_details,omitempty"`
}

func pagerDutyDedupKey(testID string) string {
	return "byot-" + testID
}

func TriggerPagerDuty(ctx context.Context, routingKey string, p Payload) error {
	return postPagerDuty(ctx, pagerDutyEvent{
		RoutingKey:  routingKey,
		EventAction: "trigger",
		DedupKey:    pagerDutyDedupKey(p.TestID),
		Payload: &pagerDutyPayload{
			Summary:   fmt.Sprintf("BYOT: %s failed %d times in a row", p.TestName, p.ConsecutiveFailures),
			Source:    "byot",
			Severity:  "critical",
			Component: p.TestName,
			CustomDetails: map[string]any{
				"test_id":              p.TestID,
				"run_id":               p.RunID,
				"consecutive_failures": p.ConsecutiveFailures,
				"failure_threshold":    p.FailureThreshold,
			},
		},
	})
}

func ResolvePagerDuty(ctx context.Context, routingKey string, p Payload) error {
	return postPagerDuty(ctx, pagerDutyEvent{
		RoutingKey:  routingKey,
		EventAction: "resolve",
		DedupKey:    pagerDutyDedupKey(p.TestID),
	})
}

func postPagerDuty(ctx context.Context, event pagerDutyEvent) error {
	if event.RoutingKey == "" {
		return nil
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, webhookTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, pagerDutyEventsURL(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// The Events API answers 202 Accepted; 400 means a bad routing key or payload.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pagerduty returned status %d", resp.StatusCode)
	}
	return nil
}
