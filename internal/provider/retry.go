package provider

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"time"
)

const defaultMaxRetries = 3

// doWithRetry executes an HTTP request, retrying on 429 responses with
// exponential backoff. It reconstructs the request body from payload on
// each retry since the reader is consumed after each attempt.
func doWithRetry(ctx context.Context, client *http.Client, req *http.Request, payload []byte, maxRetries int) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		req.Body = io.NopCloser(bytes.NewReader(payload))
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests || attempt >= maxRetries {
			return resp, nil
		}
		resp.Body.Close()

		wait := retryDelay(resp, attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

// retryDelay returns the wait duration for a retry attempt. If the response
// includes a Retry-After header with a valid number of seconds, that value
// is used. Otherwise, exponential backoff is applied: 1s, 2s, 4s, ...
func retryDelay(resp *http.Response, attempt int) time.Duration {
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return time.Duration(1<<uint(attempt)) * time.Second
}
