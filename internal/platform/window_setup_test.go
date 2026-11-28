package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowSetupFreshDismissedAndExisting(t *testing.T) {
	root := t.TempDir()
	db := filepath.Join(root, "queue.sqlite")
	if err := MarkFreshWindowSetup(root, db); err != nil || !WindowSetupPending(root) {
		t.Fatalf("fresh root must offer setup, error: %v", err)
	}
	if err := DismissWindowSetup(root); err != nil {
		t.Fatal(err)
	}
	if err := MarkFreshWindowSetup(root, db); err != nil || WindowSetupPending(root) {
		t.Fatalf("fresh boot after dismissal must keep dismissal, error: %v", err)
	}

	root = t.TempDir()
	db = filepath.Join(root, "queue.sqlite")
	if err := os.WriteFile(db, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MarkFreshWindowSetup(root, db); err != nil || WindowSetupPending(root) {
		t.Fatalf("an existing database must skip setup, error: %v", err)
	}
}
