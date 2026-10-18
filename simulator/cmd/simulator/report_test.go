package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReportKeepsLastThreeAndPreservesOtherFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.json")
	other := filepath.Join(dir, "other.json")
	if err := os.WriteFile(other, []byte("unrelated"), 0644); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 5; i++ {
		if err := writeReport(path, i); err != nil {
			t.Fatal(err)
		}
	}
	assertReportFiles(t, path, []string{"5\n", "4\n", "3\n"})
	if data, err := os.ReadFile(other); err != nil || string(data) != "unrelated" {
		t.Fatal("unrelated file changed", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 4 {
		t.Fatalf("unexpected files after rotation: %v %v", entries, err)
	}
}

func TestFailedSimulationReportRetainsPreviousSuccesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := writeReport(path, map[string]bool{"verified": true}); err != nil {
		t.Fatal(err)
	}
	if err := writeReport(path, map[string]string{"error": "upstream unavailable"}); err != nil {
		t.Fatal(err)
	}
	previous, err := os.ReadFile(reportPaths(path)[1])
	if err != nil || !strings.Contains(string(previous), `"verified": true`) {
		t.Fatal("successful report was lost", err)
	}
}

func TestReportSerializationFailureDoesNotRotateHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	for i := 1; i <= 3; i++ {
		if err := writeReport(path, i); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeReport(path, make(chan int)); err == nil {
		t.Fatal("invalid JSON value accepted")
	}
	assertReportFiles(t, path, []string{"3\n", "2\n", "1\n"})
}

func TestReportPublishingFailureRestoresAllPreviousReports(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	for i := 1; i <= 3; i++ {
		if err := writeReport(path, i); err != nil {
			t.Fatal(err)
		}
	}
	for failAt := 1; failAt <= 6; failAt++ {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			calls := 0
			injected := errors.New("publishing failed")
			err := writeReportWithRename(path, 4, func(from, to string) error {
				calls++
				if calls == failAt {
					return injected
				}
				return os.Rename(from, to)
			})
			if !errors.Is(err, injected) {
				t.Fatal("missing injected error", err)
			}
			assertReportFiles(t, path, []string{"3\n", "2\n", "1\n"})
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil || len(entries) != 3 {
				t.Fatal("temporary files remain", entries, err)
			}
		})
	}
}

func TestReportRejectsConcurrentWriterAndNonFileHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := writeReport(path, 1); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	lock.Close()
	if err := writeReport(path, 2); err == nil {
		t.Fatal("second writer ignored the lock")
	}
	if err := os.Remove(path + ".lock"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(reportPaths(path)[1], 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeReport(path, 2); err == nil {
		t.Fatal("directory used as report history")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "1\n" {
		t.Fatal("original report changed", err)
	}
}

func TestReportCompactsHistoryWithMissingCurrentFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report")
	for i := 1; i <= 3; i++ {
		if err := writeReport(path, i); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := writeReport(path, 4); err != nil {
		t.Fatal(err)
	}
	assertReportFiles(t, path, []string{"4\n", "2\n", "1\n"})
}

func assertReportFiles(t *testing.T, path string, want []string) {
	t.Helper()
	var got []string
	for _, name := range reportPaths(path) {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, string(data))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reports: got %q, want %q", got, want)
	}
}
