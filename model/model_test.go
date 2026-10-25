package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMaskWebhook(t *testing.T) {
	cases := map[string]string{
		"": "",
		"https://hooks.slack.com/services/T000/B000/XXXXsecretA1b2": "https://hooks.slack.com/…A1b2",
		"not a url": "… url",
	}
	for in, want := range cases {
		if got := MaskWebhook(in); got != want {
			t.Errorf("MaskWebhook(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTestJSONMasksWebhook(t *testing.T) {
	const secret = "https://hooks.slack.com/services/T000/B000/XXXXsecretA1b2"
	for _, v := range []any{Test{WebhookURL: secret}, &Test{WebhookURL: secret}, TestListItem{Test: Test{WebhookURL: secret}}} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "secret") {
			t.Errorf("webhook URL leaked: %s", b)
		}
		if !strings.Contains(string(b), `"webhook_url":"https://hooks.slack.com/…A1b2"`) {
			t.Errorf("masked webhook missing: %s", b)
		}
	}
}

func TestTestJSONMasksPagerDutyKey(t *testing.T) {
	const key = "R0UT1NGKEYsecretsecretsecret9z8y"
	b, err := json.Marshal(Test{PagerDutyKey: key})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret") {
		t.Errorf("routing key leaked: %s", b)
	}
	if !strings.Contains(string(b), `"pagerduty_routing_key":"…9z8y"`) {
		t.Errorf("masked key missing: %s", b)
	}
	if MaskKey("") != "" {
		t.Error("empty key should stay empty")
	}
}
