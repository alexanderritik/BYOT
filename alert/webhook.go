package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const webhookTimeout = 10 * time.Second

type Payload struct {
	TestID              string
	TestName            string
	Severity            string
	ConsecutiveFailures int
	FailureThreshold    int
	RunID               string
}

type slackBody struct {
	Text string `json:"text"`
}

func SendFailure(ctx context.Context, webhookURL string, p Payload) error {
	text := fmt.Sprintf(
		":rotating_light: *BYOT alert* · %s (%s)\n%s failed *%d* times in a row (threshold %d). Severity %s · run `%s`",
		p.TestName, p.TestID, p.TestName, p.ConsecutiveFailures, p.FailureThreshold, p.Severity, p.RunID,
	)
	return postSlack(ctx, webhookURL, text)
}

func SendRecovery(ctx context.Context, webhookURL string, p Payload) error {
	text := fmt.Sprintf(
		":white_check_mark: *BYOT recovered* · %s (%s)\nLatest run passed after consecutive failures. Severity %s.",
		p.TestName, p.TestID, p.Severity,
	)
	return postSlack(ctx, webhookURL, text)
}

func postSlack(ctx context.Context, webhookURL, text string) error {
	if webhookURL == "" {
		return nil
	}
	body, err := json.Marshal(slackBody{Text: text})
	if err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx, webhookTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
