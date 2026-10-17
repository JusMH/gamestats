package simulation

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.HandlerFunc, timeout time.Duration) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(server.URL, timeout)
	if err != nil {
		t.Fatal(err)
	}
	return client
}
func TestClientValidation(t *testing.T) {
	for _, url := range []string{"", "/relative", "ftp://host", "http://user:pass@host", "http://host?x=1", "http://host#x"} {
		t.Run(url, func(t *testing.T) {
			if _, err := NewClient(url, time.Second); err == nil {
				t.Fatal("invalid URL accepted")
			}
		})
	}
	if _, err := NewClient("http://host", 0); err == nil {
		t.Fatal("zero timeout accepted")
	}
}
func TestClientPublicWireAndBasePath(t *testing.T) {
	e := mustScenarios(t, "wire")[0].Events[0]
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prefix/events" || r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("wire mismatch")
		}
		_, _ = w.Write([]byte(`{"status":"accepted","eventId":"` + e.EventID + `"}`))
	}, time.Second)
	c.base += "/prefix"
	if err := c.Send(context.Background(), e); err != nil {
		t.Fatal(err)
	}
}
func TestHTTPFailuresNeverRetry(t *testing.T) {
	for _, code := range []int{400, 404, 409, 500, 502, 503, 504} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			calls := 0
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { calls++; http.Error(w, "test error", code) }, time.Second)
			err := c.Send(context.Background(), mustScenarios(t, "error")[0].Events[0])
			var failure *HTTPError
			if !errors.As(err, &failure) || failure.Status != code || calls != 1 {
				t.Fatal("wrong error/retry", err, calls)
			}
			if code == 409 && !strings.Contains(err.Error(), "--run-id") {
				t.Fatal("missing rerun guidance")
			}
		})
	}
}
func TestMalformedResponses(t *testing.T) {
	for _, body := range []string{"", `{}`, `null`, `{"status":"stored","eventId":"x"}`, `{"status":"accepted","eventId":"other"}`, `{"status":"accepted"} {}`} {
		t.Run(body, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }, time.Second)
			if err := c.Send(context.Background(), mustScenarios(t, "bad")[0].Events[0]); err == nil {
				t.Fatal("bad acknowledgment accepted")
			}
		})
	}
}
func TestRedirectNotFollowed(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { calls++; http.Redirect(w, r, "/other", 302) }, time.Second)
	if err := c.Send(context.Background(), mustScenarios(t, "redirect")[0].Events[0]); expectHTTP(err, 302) != nil || calls != 1 {
		t.Fatal(err, calls)
	}
}
func TestResponseLimit(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", maxResponse+1)))
	}, time.Second)
	if err := c.Exchange(context.Background(), "GET", "/results", nil, nil); err == nil || !strings.Contains(err.Error(), "8 MiB") {
		t.Fatal(err)
	}
}
func TestTimeoutAndCancellation(t *testing.T) {
	for _, cancelFirst := range []bool{false, true} {
		t.Run(map[bool]string{true: "cancel", false: "timeout"}[cancelFirst], func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = io.Copy(io.Discard, r.Body); <-r.Context().Done() }, 40*time.Millisecond)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelFirst {
				cancel()
			}
			err := c.Send(ctx, mustScenarios(t, "timeout")[0].Events[0])
			want := context.DeadlineExceeded
			if cancelFirst {
				want = context.Canceled
			}
			if !errors.Is(err, want) {
				t.Fatalf("got %v want %v", err, want)
			}
		})
	}
}
