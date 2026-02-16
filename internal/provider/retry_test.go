package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoWithRetrySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", server.URL, nil)
	resp, err := doWithRetry(context.Background(), http.DefaultClient, req, []byte(`{}`), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestDoWithRetry429ThenSuccess(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", server.URL, nil)
	resp, err := doWithRetry(context.Background(), http.DefaultClient, req, []byte(`{}`), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if calls.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", calls.Load())
	}
}

func TestDoWithRetryExhausted(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	req, _ := http.NewRequestWithContext(context.Background(), "POST", server.URL, nil)
	resp, err := doWithRetry(context.Background(), http.DefaultClient, req, []byte(`{}`), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
	// 1 initial + 3 retries = 4 total calls
	if calls.Load() != 4 {
		t.Errorf("expected 4 calls, got %d", calls.Load())
	}
}

func TestDoWithRetryRespectsContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req, _ := http.NewRequestWithContext(ctx, "POST", server.URL, nil)
	_, err := doWithRetry(ctx, http.DefaultClient, req, []byte(`{}`), 3)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestRetryDelayRetryAfterHeader(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "5")
	d := retryDelay(resp, 0)
	if d != 5*time.Second {
		t.Errorf("expected 5s, got %v", d)
	}
}

func TestRetryDelayExponentialBackoff(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	cases := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
	}
	for _, tc := range cases {
		d := retryDelay(resp, tc.attempt)
		if d != tc.expected {
			t.Errorf("attempt %d: expected %v, got %v", tc.attempt, tc.expected, d)
		}
	}
}
