package simulation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const maxResponse = 8 << 20

type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	hint := ""
	if e.Status == http.StatusConflict {
		hint = "; IDs already used or match closed: choose a new --run-id or restart Go; no retries performed"
	}
	return fmt.Sprintf("HTTP %d: %s%s", e.Status, e.Body, hint)
}

type Client struct {
	base   string
	http   *http.Client
	active atomic.Int64
	peak   atomic.Int64
}

func NewClient(base string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("base URL must be absolute HTTP(S), without credentials, query or fragment")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be positive")
	}
	return &Client{base: strings.TrimRight(base, "/"), http: &http.Client{
		Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// Exchange does not retry: an uncertain POST may already have been accepted.
func (c *Client) Exchange(ctx context.Context, method, route string, body []byte, target any) error {
	current := c.active.Add(1)
	defer c.active.Add(-1)
	for old := c.peak.Load(); current > old; old = c.peak.Load() {
		if c.peak.CompareAndSwap(old, current) {
			break
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+route, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, route, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxResponse {
		return fmt.Errorf("response exceeds 8 MiB")
	}
	if response.StatusCode != http.StatusOK {
		message := string(data)
		if len(message) > 512 {
			message = message[:512]
		}
		return &HTTPError{response.StatusCode, message}
	}
	if !json.Valid(data) {
		return fmt.Errorf("%s %s: invalid JSON response", method, route)
	}
	if target != nil {
		if err := json.Unmarshal(data, target); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
func (c *Client) Send(ctx context.Context, e Event) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	var ack struct {
		Status  string `json:"status"`
		EventID string `json:"eventId"`
	}
	if err := c.Exchange(ctx, http.MethodPost, "/events", data, &ack); err != nil {
		return err
	}
	if ack.Status != "accepted" || ack.EventID != e.EventID {
		return fmt.Errorf("unexpected acceptance for event %s: %+v", e.EventID, ack)
	}
	return nil
}
func route(kind string, e Event) string {
	return "/" + kind + "/" + url.PathEscape(e.GameID) + "/" + url.PathEscape(e.MatchID)
}
func (c *Client) Results(ctx context.Context, e Event) (Result, error) {
	var r Result
	err := c.Exchange(ctx, http.MethodGet, route("results", e), nil, &r)
	return r, err
}
func (c *Client) Match(ctx context.Context, e Event) (Match, error) {
	var m Match
	err := c.Exchange(ctx, http.MethodGet, route("matches", e), nil, &m)
	return m, err
}
