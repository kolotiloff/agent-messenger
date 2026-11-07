package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWindowFileForwardsAndIsPrivate(t *testing.T) {
	root := t.TempDir()
	address := "http://127.0.0.1:4242/#0123abcd"
	path, err := writeWindowFile(root, address)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `content="0;url=`+address+`"`) {
		t.Fatalf("page %s", raw)
	}
	if info, err := os.Stat(path); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("mode %v %v", info.Mode(), err)
	}
	// Only regular forwarding files are removed.
	other := filepath.Join(root, "queue.sqlite")
	os.WriteFile(other, nil, 0o600)
	removeOldWindowFiles(root)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("old forwarding file kept")
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatal("other file removed")
	}
}

func TestWindowOpenerRefusesOtherAddresses(t *testing.T) {
	o := &windowOpener{root: t.TempDir(), out: &strings.Builder{}}
	if _, err := o.Write([]byte("http://example.com/#x\n")); err == nil {
		t.Fatal("foreign address accepted")
	}
}

func TestOpenTakesNoArguments(t *testing.T) {
	if err := runOpen(t.TempDir(), []string{"--allow-model-calls"}, &strings.Builder{}); err == nil {
		t.Fatal("arguments accepted")
	}
}
