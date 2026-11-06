package distribution

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPinnedRuntimeSurvivesBundleReplacementAndRejectsCorruption(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "bundled")
	os.WriteFile(source, []byte("version one"), 0700)
	first, err := CacheRuntime(source, filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(source, []byte("version two"), 0700)
	second, err := CacheRuntime(source, filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("running runtime replaced")
	}
	old, _ := os.ReadFile(first)
	if string(old) != "version one" {
		t.Fatal("old bytes lost")
	}
	again, err := CacheRuntime(source, filepath.Join(dir, "cache"))
	if err != nil || again != second {
		t.Fatal("not idempotent", err)
	}
	os.WriteFile(second, []byte("corrupted"), 0700)
	if _, err := CacheRuntime(source, filepath.Join(dir, "cache")); err == nil {
		t.Fatal("corrupted cache reused")
	}
}
