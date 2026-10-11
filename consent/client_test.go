package consent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGrantRequestAndRetry(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/auth/consent/grant/user-7" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization header not set")
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["category"] != PaymentCategory || body["idempotency_key"] != "decision-9" {
			t.Errorf("request body = %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"retry","message":"retry later"},"metadata":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"granted":true},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := Client{
		BaseURL: server.URL, APIKey: "test-key", HTTPClient: server.Client(), MaxRetries: 1,
		Sleep: func(context.Context, time.Duration) error { return nil },
	}
	if err := client.Grant(context.Background(), "user-7", PaymentCategory, "checkout", "decision-9"); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}
