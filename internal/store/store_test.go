package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"time-tracking/internal/model"
)

func TestPersistRevisionAndInterruptedSubmission(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	d := model.Draft{ID: "one", Date: "2026-10-05", Minutes: 80, Status: "draft"}
	if err = s.SaveDraft(&d, 0); err != nil {
		t.Fatal(err)
	}
	stale := d
	d.Status = "submitting"
	if err = s.SaveDraft(&d, d.Revision); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveDraft(&stale, stale.Revision); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Draft("one")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "uncertain" || got.Minutes != 80 || got.Revision != 3 {
		t.Fatalf("recovery failed: %+v", got)
	}
}
func TestBackupAndLogReplacement(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = os.WriteFile(filepath.Join(dir, "credentials.bin"), []byte("TOP-SECRET"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.ReplaceLogs("2026-10-05", "2026-10-11", []model.Log{{ID: "remote", Date: "2026-10-05", Minutes: 80}}); err != nil {
		t.Fatal(err)
	}
	if err = s.ReplaceLogs("2026-10-05", "2026-10-11", []model.Log{}); err != nil {
		t.Fatal(err)
	}
	logs, err := s.Logs("2026-10-05", "2026-10-11")
	if err != nil || len(logs) != 0 {
		t.Fatal(logs, err)
	}
	raw, err := s.Backup()
	if err != nil {
		t.Fatal(err)
	}
	if string(raw[:15]) != "SQLite format 3" {
		t.Fatal("not a SQLite backup")
	}
	if bytes.Contains(raw, []byte("TOP-SECRET")) {
		t.Fatal("backup leaked secrets")
	}
}
