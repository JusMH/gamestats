package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelpAndInvalidCLI(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		code int
	}{
		{"help", []string{"--help"}, 0},
		{"mode", []string{"--mode", "invalid"}, 2},
		{"limit", []string{"--concurrency", "1"}, 2},
		{"run", []string{"--run-id", "../bad"}, 2},
		{"url", []string{"--go-url", "file:///x"}, 2},
		{"timeout", []string{"--timeout", "0s"}, 2},
		{"duration", []string{"--max-duration", "0s"}, 2},
		{"negative_verify", []string{"--mode", "negative", "--verify=false"}, 2},
		{"argument", []string{"extra"}, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, errout bytes.Buffer
			if code := run(context.Background(), test.args, &out, &errout); code != test.code {
				t.Fatalf("got %d want %d: %s", code, test.code, errout.String())
			}
		})
	}
}
func TestCLIFailsOnHTTP200WithWrongAnalyticsAndWritesReport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("{}")) }))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "evidence", "report.json")
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"--go-url", server.URL, "--report", path}, &out, &errout); code != 1 {
		t.Fatal(code)
	}
	data, err := os.ReadFile(path)
	if err != nil || !json.Valid(data) {
		t.Fatal("missing failure report", err)
	}
	if !strings.Contains(out.String(), "SUMMARY") || !strings.Contains(errout.String(), "FAILED") {
		t.Fatal("missing diagnostics")
	}
}
