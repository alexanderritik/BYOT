package secrets

import (
	"encoding/base64"
	"strings"
	"testing"
)

func testBox(t *testing.T) *Box {
	t.Helper()
	b, err := New(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))))
	if err != nil || b == nil {
		t.Fatalf("New: %v", err)
	}
	return b
}

func TestRoundTrip(t *testing.T) {
	b := testBox(t)
	enc, err := b.Encrypt("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(enc, "hunter2") || !strings.HasPrefix(enc, "v1:") {
		t.Fatalf("bad ciphertext %q", enc)
	}
	enc2, _ := b.Encrypt("hunter2")
	if enc == enc2 {
		t.Fatal("nonce reused")
	}
	got, err := b.Decrypt(enc)
	if err != nil || got != "hunter2" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestWrongKeyAndDisabled(t *testing.T) {
	enc, _ := testBox(t).Encrypt("x")
	other, _ := New(base64.StdEncoding.EncodeToString([]byte(strings.Repeat("z", 32))))
	if _, err := other.Decrypt(enc); err == nil {
		t.Fatal("expected decrypt failure with wrong key")
	}
	var nilBox *Box
	if _, err := nilBox.Encrypt("x"); err != ErrNoKey {
		t.Fatalf("got %v", err)
	}
	if b, err := New(""); b != nil || err != nil {
		t.Fatal("empty key should disable secrets")
	}
	if _, err := New(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("expected length error")
	}
}

func TestRedact(t *testing.T) {
	out := Redact([]byte("tok=abc123 b64="+base64.StdEncoding.EncodeToString([]byte("abc123"))+" ok"), []string{"abc123", ""})
	if strings.Contains(string(out), "abc123") || strings.Contains(string(out), "YWJjMTIz") || !strings.Contains(string(out), "ok") {
		t.Fatalf("got %q", out)
	}
}

func TestValidateName(t *testing.T) {
	for _, n := range []string{"BASE_URL", "_x1"} {
		if ValidateName(n) != nil {
			t.Errorf("%s should be valid", n)
		}
	}
	for _, n := range []string{"", "1A", "A-B", "PATH", "path", "DOCKER_HOST", "ld_preload", strings.Repeat("A", 65)} {
		if ValidateName(n) == nil {
			t.Errorf("%q should be invalid", n)
		}
	}
}
