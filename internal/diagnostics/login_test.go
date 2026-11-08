package diagnostics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each agent's own sign-out, next to its sign-in (T-056).
func TestLogoutCommand(t *testing.T) {
	cli := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(cli, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	for provider, want := range map[string]string{"claude": "auth logout", "codex": "logout"} {
		argv, _, err := LogoutCommand(provider, cli, "")
		if err != nil || strings.Join(argv[1:], " ") != want {
			t.Fatalf("%s: %v %v", provider, argv, err)
		}
	}
	if _, _, err := LogoutCommand("gemini", cli, ""); err == nil {
		t.Fatal("unknown agent accepted")
	}
}
