package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Each explicitly requested report has one current file and two backups.
// Other report names and unrelated files in the directory are never pruned.
func reportPaths(path string) []string {
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	return []string{path, stem + ".1" + ext, stem + ".2" + ext}
}

func writeReport(path string, value any) error {
	return writeReportWithRename(path, value, os.Rename)
}

// The rename argument allows tests to verify rollback after a publishing error.
// Existing reports are staged reversibly, and pruned only after the new JSON
// and both retained backups have been installed successfully.
func writeReportWithRename(path string, value any, rename func(string, string) error) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	lockPath := path + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("cannot lock report %s (another writer or an interrupted write): %w", path, err)
	}
	defer func() {
		lock.Close()
		os.Remove(lockPath)
	}()
	paths := reportPaths(path)
	var existing []string
	for _, candidate := range paths {
		info, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("report history must contain regular files: %s", candidate)
		}
		existing = append(existing, candidate)
	}
	transaction, err := os.MkdirTemp(directory, "."+filepath.Base(path)+".rotate-")
	if err != nil {
		return err
	}
	staged := filepath.Join(transaction, "new")
	// Remove only our own new staging file and the transaction directory if empty.
	// A backup that could not be restored is deliberately retained for recovery.
	defer func() {
		os.Remove(staged)
		os.Remove(transaction)
	}()
	file, err := os.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(append(data, '\n'))
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	type move struct{ from, to string }
	var applied []move
	apply := func(from, to string) error {
		if err := rename(from, to); err != nil {
			return err
		}
		applied = append(applied, move{from, to})
		return nil
	}
	rollback := func(cause error) error {
		for i := len(applied) - 1; i >= 0; i-- {
			item := applied[i]
			if err := rename(item.to, item.from); err != nil {
				cause = errors.Join(cause, fmt.Errorf("report rollback failed; inspect %s: %w", transaction, err))
			}
		}
		return cause
	}
	for i, source := range existing {
		if err := apply(source, filepath.Join(transaction, fmt.Sprintf("old-%d", i))); err != nil {
			return rollback(err)
		}
	}
	if err := apply(staged, paths[0]); err != nil {
		return rollback(err)
	}
	for i := 0; i < len(existing) && i < 2; i++ {
		if err := apply(filepath.Join(transaction, fmt.Sprintf("old-%d", i)), paths[i+1]); err != nil {
			return rollback(err)
		}
	}
	if len(existing) == 3 {
		if err := os.Remove(filepath.Join(transaction, "old-2")); err != nil {
			return fmt.Errorf("new report saved, but oldest report cleanup failed in %s: %w", transaction, err)
		}
	}
	return nil
}
