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

func TestLoginAllowsMissingDefaultProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cli := filepath.Join(home, "fake-cli.exe")
	if err := os.WriteFile(cli, []byte("fixture; never run"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, provider := range []string{"claude", "codex", "cursor"} {
		profile := filepath.Join(home, "."+provider)
		argv, env, err := LoginCommand(provider, cli, profile)
		if err != nil || argv[0] != cli || argv[len(argv)-1] != "login" || len(env) != 0 {
			t.Fatalf("first login %s: %v %v %v", provider, argv, env, err)
		}
		if _, err := os.Stat(profile); !os.IsNotExist(err) {
			t.Fatalf("Messenger created profile %s", profile)
		}
		missing := filepath.Join(home, "missing-custom-"+provider)
		if _, _, err := LoginCommand(provider, cli, missing); err == nil || !strings.Contains(err.Error(), missing) {
			t.Fatalf("missing custom profile silently replaced: %v", err)
		}
		custom := filepath.Join(home, "custom-"+provider)
		os.Mkdir(custom, 0700)
		canonical, err := filepath.EvalSymlinks(custom)
		if err != nil {
			t.Fatal(err)
		}
		if _, env, err := LoginCommand(provider, cli, custom); err != nil || len(env) != 1 || !strings.HasSuffix(env[0], "="+canonical) {
			t.Fatalf("custom profile lost: %v %v", env, err)
		}
		// A non-directory at the default path is an error, not a first login.
		os.WriteFile(profile, []byte("not a directory"), 0600)
		if _, _, err := LoginCommand(provider, cli, profile); err == nil {
			t.Fatal("non-directory default profile accepted")
		}
	}
}
