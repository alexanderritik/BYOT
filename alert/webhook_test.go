package alert

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendFailurePostsSlackJSON(t *testing.T) {
	var got slackBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := SendFailure(context.Background(), srv.URL, Payload{
		TestID:              "abc",
		TestName:            "checkout",
		Severity:            "P0",
		ConsecutiveFailures: 3,
		FailureThreshold:    3,
		RunID:               "run-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text == "" {
		t.Fatal("expected text payload")
	}
}
