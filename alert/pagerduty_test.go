package alert

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakePagerDuty records events and answers like the Events API.
func fakePagerDuty(t *testing.T, status int) *[]pagerDutyEvent {
	t.Helper()
	var got []pagerDutyEvent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var ev pagerDutyEvent
		if err := json.Unmarshal(b, &ev); err != nil {
			t.Fatal(err)
		}
		got = append(got, ev)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PAGERDUTY_EVENTS_URL", srv.URL)
	return &got
}

func TestPagerDutyTriggerThenResolveShareDedupKey(t *testing.T) {
	got := fakePagerDuty(t, http.StatusAccepted)
	p := Payload{TestID: "abc", TestName: "checkout", ConsecutiveFailures: 3, FailureThreshold: 3, RunID: "run-1"}

	if err := TriggerPagerDuty(context.Background(), "key123", p); err != nil {
		t.Fatal(err)
	}
	if err := ResolvePagerDuty(context.Background(), "key123", p); err != nil {
		t.Fatal(err)
	}

	if len(*got) != 2 {
		t.Fatalf("want 2 events, got %d", len(*got))
	}
	trigger, resolve := (*got)[0], (*got)[1]
	if trigger.EventAction != "trigger" || resolve.EventAction != "resolve" {
		t.Fatalf("actions: %q, %q", trigger.EventAction, resolve.EventAction)
	}
	if trigger.DedupKey != "byot-abc" || resolve.DedupKey != trigger.DedupKey {
		t.Fatalf("dedup keys: %q, %q", trigger.DedupKey, resolve.DedupKey)
	}
	if trigger.RoutingKey != "key123" || trigger.Payload == nil || trigger.Payload.Severity != "critical" {
		t.Fatalf("trigger: %+v", trigger)
	}
	if !strings.Contains(trigger.Payload.Summary, "checkout") {
		t.Fatalf("summary: %q", trigger.Payload.Summary)
	}
	if resolve.Payload != nil {
		t.Fatal("resolve must not carry a payload")
	}
}

func TestPagerDutyRejectedKeyIsAnError(t *testing.T) {
	fakePagerDuty(t, http.StatusBadRequest)
	if err := TriggerPagerDuty(context.Background(), "bad", Payload{TestID: "abc"}); err == nil {
		t.Fatal("expected error for 400")
	}
}

func TestNotifyFailureSkipsUnsetChannelsAndKeepsGoing(t *testing.T) {
	got := fakePagerDuty(t, http.StatusAccepted)
	p := Payload{TestID: "abc", TestName: "checkout", ConsecutiveFailures: 3}

	if err := NotifyFailure(context.Background(), Targets{}, p); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 0 {
		t.Fatal("no channels configured, nothing should be sent")
	}

	// A broken webhook still lets the PagerDuty event through.
	err := NotifyFailure(context.Background(), Targets{WebhookURL: "http://127.0.0.1:1", PagerDutyKey: "key123"}, p)
	if err == nil || !strings.Contains(err.Error(), "webhook") {
		t.Fatalf("want webhook error, got %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("pagerduty event not sent: %d", len(*got))
	}
}
