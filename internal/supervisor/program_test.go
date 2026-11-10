package supervisor

import (
	"os"
	"path/filepath"
	"testing"
)

// D-118: workers start the pinned copy only when it is this very program.
func TestWorkerProgramUsesOnlyMatchingPinnedCopy(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	same := filepath.Join(dir, "same")
	other := filepath.Join(dir, "other")
	os.WriteFile(same, body, 0o700)
	os.WriteFile(other, []byte("another version"), 0o700)
	for _, c := range []struct{ pinned, want string }{
		{same, same},
		{other, self},
		{"", self},
		{"relative", self},
		{filepath.Join(dir, "missing"), self},
		{dir, self},
	} {
		got, err := resolveWorkerProgram(c.pinned)
		if err != nil || got != c.want {
			t.Fatalf("pinned %q: got %q, %v", c.pinned, got, err)
		}
	}
}
