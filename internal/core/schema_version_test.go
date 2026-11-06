package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaVersionIsRecordedAndNewerIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.sqlite")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var v string
	if err := st.db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&v); err != nil || v != "2" {
		t.Fatalf("version %q %v", v, err)
	}
	if _, err := st.db.Exec(`UPDATE meta SET value='99' WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	st.Close()
	before, _ := os.ReadFile(path)
	if _, err := Open(path); !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("newer database opened: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("refused database was modified")
	}
}

func TestSchemaVersionDamagedIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.sqlite")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE meta SET value='x' WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if _, err := Open(path); !errors.Is(err, ErrRejected) {
		t.Fatalf("damaged version accepted: %v", err)
	}
}
