// Package analysis provides the HTTP bridge to the independent Scala service.
package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gamestats/go-ingestion/internal/matches"
)

const DefaultTimeout = 5 * time.Second
const maxResponseBytes = 8 << 20

var ErrInvalidResponse = errors.New("invalid analysis response")

// Result leaves metric and pattern semantics entirely to Scala. RawMessage
// preserves arbitrary nested JSON and numbers without float64 conversion.
type Result struct {
	GameID   string          `json:"gameId"`
	MatchID  string          `json:"matchId"`
	State    string          `json:"state"`
	Metrics  json.RawMessage `json:"metrics"`
	Patterns json.RawMessage `json:"patterns"`
}

// Client is reusable across concurrent requests; configuration is immutable.
type Client struct {
	endpoint string
	http     *http.Client
}

// NewClient accepts a service base URL. A zero timeout selects DefaultTimeout.
// Redirects are rejected: analysis payloads must only reach the configured URL.
func NewClient(baseURL string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("analysis URL must be an absolute HTTP(S) base URL without credentials, query or fragment")
	}
	if timeout < 0 {
		return nil, errors.New("analysis timeout must not be negative")
	}
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	endpoint := strings.TrimRight(u.String(), "/") + "/analyze"
	return &Client{endpoint: endpoint, http: &http.Client{
		Timeout:       timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *Client) Analyze(ctx context.Context, snapshot matches.EventSnapshot) (Result, error) {
	var result Result
	body, err := json.Marshal(snapshot)
	if err != nil {
		return result, fmt.Errorf("encode snapshot: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return result, fmt.Errorf("request analysis: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("analysis HTTP status %d", response.StatusCode)
	}
	body, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return result, fmt.Errorf("read analysis: %w", err)
	}
	if len(body) > maxResponseBytes {
		return result, ErrInvalidResponse
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return Result{}, ErrInvalidResponse
	}
	if result.GameID != snapshot.GameID || result.MatchID != snapshot.MatchID || result.State != snapshot.State || !isArray(result.Metrics) || !isArray(result.Patterns) {
		return Result{}, ErrInvalidResponse
	}
	return result, nil
}

func isArray(value json.RawMessage) bool {
	value = bytes.TrimSpace(value)
	return len(value) > 0 && value[0] == '['
}
