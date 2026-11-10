package distribution

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func pinnedCopy(t *testing.T, dir, body string) string {
	t.Helper()
	name := "bundled"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	source := filepath.Join(dir, name)
	if err := os.WriteFile(source, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	pinned, err := CacheRuntime(source, filepath.Join(dir, "runtimes"))
	if err != nil {
		t.Fatal(err)
	}
	return pinned
}

// D-118: every version runs from the same path; the pinned copies stay.
func TestStableRuntimeKeepsOnePathAcrossVersions(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "runtimes")
	first := pinnedCopy(t, dir, "version one")
	stable1, release, err := StableRuntime(first, cache)
	if err != nil {
		t.Fatal(err)
	}
	release()
	second := pinnedCopy(t, dir, "version two")
	stable2, release, err := StableRuntime(second, cache)
	if err != nil {
		t.Fatal(err)
	}
	release()
	if stable1 != stable2 || filepath.Dir(stable1) != cache {
		t.Fatalf("stable path moved: %s, %s", stable1, stable2)
	}
	if b, _ := os.ReadFile(stable2); string(b) != "version two" {
		t.Fatalf("stable file holds %q", b)
	}
	if b, _ := os.ReadFile(first); string(b) != "version one" {
		t.Fatal("pinned copy of the old version changed")
	}
	again, release, err := StableRuntime(second, cache)
	if err != nil || again != stable2 {
		t.Fatal("not idempotent", err)
	}
	release()
	entries, _ := os.ReadDir(cache)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".old-") || strings.HasPrefix(e.Name(), ".stable-") {
			t.Fatalf("left behind %s", e.Name())
		}
	}
}

func TestStableRuntimeIsHeldByOneStart(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "runtimes")
	pinned := pinnedCopy(t, dir, "version one")
	_, release, err := StableRuntime(pinned, cache)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := StableRuntime(pinned, cache); !errors.Is(err, ErrStableBusy) {
		t.Fatalf("second start got %v", err)
	}
	release()
	_, release, err = StableRuntime(pinned, cache)
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestStableRuntimeReplacesDamagedFile(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "runtimes")
	pinned := pinnedCopy(t, dir, "version one")
	stable, release, err := StableRuntime(pinned, cache)
	if err != nil {
		t.Fatal(err)
	}
	release()
	if err := os.WriteFile(stable, []byte("damaged"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, release, err = StableRuntime(pinned, cache)
	if err != nil {
		t.Fatal(err)
	}
	release()
	if b, _ := os.ReadFile(stable); string(b) != "version one" {
		t.Fatalf("stable file holds %q", b)
	}
}
