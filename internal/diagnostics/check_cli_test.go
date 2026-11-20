package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/cliprobe"
)

func checkFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	cli := filepath.Join(dir, "chosen-cli")
	if err := os.WriteFile(cli, []byte("explicit fixture, never executed"), 0o700); err != nil {
		t.Fatal(err)
	}
	return cli, dir
}

func TestCheckCLIExactCommandsAndProfile(t *testing.T) {
	for _, tc := range []struct {
		provider, version, variable string
		args                        []string
		output                      cliprobe.Output
	}{
		{"claude", "2.1.318 (Claude Code)", "CLAUDE_CONFIG_DIR", []string{"auth", "status", "--json"}, cliprobe.Output{Stdout: []byte(`{"loggedIn":true,"email":"PRIVATE_EMAIL","orgId":"PRIVATE_ORG"}`)}},
		{"codex", "codex-cli 0.155.1", "CODEX_HOME", []string{"login", "status"}, cliprobe.Output{Stderr: []byte("Logged in using ChatGPT\n")}},
		{"cursor", "2026.11.05-86047df", "CURSOR_CONFIG_DIR", []string{"status", "--format", "json"}, cliprobe.Output{Stdout: []byte(`{"isAuthenticated":true,"userInfo":{"email":"PRIVATE_EMAIL"}}`)}},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			cli, profile := checkFixture(t)
			t.Setenv(tc.variable, "/wrong/inherited/profile")
			calls := 0
			var cwd string
			runner := func(ctx context.Context, path, dir string, env, args []string) cliprobe.Output {
				calls++
				cwd = dir
				resolved, _ := filepath.EvalSymlinks(cli)
				if path != resolved || dir == profile || !strings.HasPrefix(filepath.Base(dir), "messenger-check-") {
					t.Fatal("unexpected execution binding", path, dir)
				}
				actual, _ := filepath.EvalSymlinks(profile)
				matches := 0
				for _, item := range env {
					if strings.HasPrefix(item, tc.variable+"=") {
						matches++
						if item != tc.variable+"="+actual {
							t.Fatal("wrong profile selected")
						}
					}
				}
				if matches != 1 {
					t.Fatal("missing or duplicated profile binding")
				}
				if calls == 1 {
					if !reflect.DeepEqual(args, []string{"--version"}) {
						t.Fatal(args)
					}
					return cliprobe.Output{Stdout: []byte(tc.version)}
				}
				if !reflect.DeepEqual(args, tc.args) {
					t.Fatal(args)
				}
				return tc.output
			}
			r, err := checkCLI(context.Background(), tc.provider, cli, profile, "darwin", runner)
			if err != nil || calls != 2 || r.Authentication.State != "authenticated" || r.Session != "not_checked" || r.Compatibility != "not_checked" {
				t.Fatal(r, err, calls)
			}
			encoded, _ := json.Marshal(r)
			if strings.Contains(string(encoded), "PRIVATE_") {
				t.Fatal("account data escaped parser")
			}
			if _, err := os.Stat(cwd); !os.IsNotExist(err) {
				t.Fatal("temporary cwd retained")
			}
		})
	}
}

func TestCheckCLIStopsBeforeAuthForUnknownVersionOrFailure(t *testing.T) {
	cli, profile := checkFixture(t)
	for _, version := range []cliprobe.Output{
		{Stdout: []byte("PRIVATE_TOKEN")},
		{Stdout: []byte("2.1.318 (Claude Code)"), Exit: 1},
		{Stdout: []byte("2.1.318 (Claude Code)"), Failure: "output_limit"},
		{Failure: "timeout_or_cancelled"},
	} {
		calls := 0
		r, err := checkCLI(context.Background(), "claude", cli, profile, "darwin", func(context.Context, string, string, []string, []string) cliprobe.Output { calls++; return version })
		if err != nil || calls != 1 || r.Authentication.State != "not_checked" {
			t.Fatal(r, err, calls)
		}
		encoded, _ := json.Marshal(r)
		if strings.Contains(string(encoded), "PRIVATE_TOKEN") {
			t.Fatal("unknown version output leaked")
		}
	}
}

func TestCheckCLIRejectsImplicitPathsAndUnsupportedPlatform(t *testing.T) {
	cli, profile := checkFixture(t)
	runner := func(context.Context, string, string, []string, []string) cliprobe.Output {
		t.Fatal("should not run CLI")
		return cliprobe.Output{}
	}
	for _, args := range [][3]string{{"claude", "claude", profile}, {"claude", cli, "relative"}, {"other", cli, profile}, {"claude", cli, filepath.Join(profile, "missing")}} {
		if _, err := checkCLI(context.Background(), args[0], args[1], args[2], "darwin", runner); err == nil {
			t.Fatal("invalid binding accepted")
		}
	}
}

// Windows and Linux run the same two requests, and a newer version still gets
// the login request (D-051).
func TestCheckCLIOnEveryPlatformAndNewerVersion(t *testing.T) {
	cli, profile := checkFixture(t)
	for _, goos := range []string{"windows", "linux", "darwin"} {
		var calls [][]string
		r, err := checkCLI(context.Background(), "claude", cli, profile, goos, func(_ context.Context, _ string, _ string, _ []string, args []string) cliprobe.Output {
			calls = append(calls, args)
			if args[0] == "--version" {
				return cliprobe.Output{Stdout: []byte("2.1.999 (Claude Code)\n")}
			}
			return cliprobe.Output{Stdout: []byte(`{"loggedIn":true}`)}
		})
		if err != nil || len(calls) != 2 || r.Version.Value != "2.1.999" || r.Authentication.State != "authenticated" || r.OS != goos {
			t.Fatal(goos, r, err, calls)
		}
	}
}

func TestAuthenticationRequiresPositiveEvidenceAndNeverLeaksSecrets(t *testing.T) {
	for _, tc := range []struct {
		provider string
		output   cliprobe.Output
		state    string
	}{
		{"claude", cliprobe.Output{Stdout: []byte(`{"loggedIn":false}`), Exit: 1}, "not_authenticated"},
		{"claude", cliprobe.Output{Stdout: []byte(`{"loggedIn":true}`), Exit: 1}, "unknown"},
		{"claude", cliprobe.Output{Stdout: []byte(`{}`)}, "unknown"},
		{"claude", cliprobe.Output{Stdout: []byte(`{"loggedIn":"true"}`)}, "unknown"},
		{"claude", cliprobe.Output{Stdout: []byte(`{"loggedIn":null}`)}, "unknown"},
		{"claude", cliprobe.Output{Stdout: []byte(`{"loggedIn":true}`), Failure: "timeout_or_cancelled"}, "unknown"},
		{"cursor", cliprobe.Output{Stdout: []byte(`{"isAuthenticated":false}`), Exit: 1}, "unknown"},
		{"codex", cliprobe.Output{Stderr: []byte("Logged in using an API key - PRIVATE_KEY")}, "authenticated"},
		{"codex", cliprobe.Output{Stderr: []byte("Logged in using ChatGPT\nPRIVATE_WARNING")}, "unknown"},
		{"codex", cliprobe.Output{Stderr: []byte("Not logged in"), Exit: 1}, "not_authenticated"},
		{"codex", cliprobe.Output{Stderr: []byte("Logged in using ChatGPT\n")}, "authenticated"},
		{"codex", cliprobe.Output{Stderr: []byte("PRIVATE_NETWORK_ERROR"), Exit: 1}, "unknown"},
	} {
		r := parseAuthentication(tc.provider, tc.output)
		if r.State != tc.state {
			t.Fatal(tc.provider, r, tc.state)
		}
		// The card names the sign-in method for Codex (T-061).
		if tc.provider == "codex" && r.State == "authenticated" {
			want := "api_key"
			if strings.Contains(string(tc.output.Stderr), "ChatGPT") {
				want = "chatgpt"
			}
			if r.Value != want {
				t.Fatal("codex method", r.Value)
			}
		}
		encoded, _ := json.Marshal(r)
		if strings.Contains(string(encoded), "PRIVATE") {
			t.Fatal("secret leaked")
		}
	}
}

func TestCheckCLIRejectsExecutableReplacement(t *testing.T) {
	cli, profile := checkFixture(t)
	calls := 0
	r, err := checkCLI(context.Background(), "claude", cli, profile, "darwin", func(context.Context, string, string, []string, []string) cliprobe.Output {
		calls++
		if err := os.WriteFile(cli, []byte("changed size and content"), 0o700); err != nil {
			t.Fatal(err)
		}
		return cliprobe.Output{Stdout: []byte("2.1.318 (Claude Code)")}
	})
	if err != nil || calls != 1 || r.Authentication.Reason != "executable_changed" {
		t.Fatal(r, err, calls)
	}
}

func TestProbeRunnerLimitsOutputAndTime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell fixture")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "fixture")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nwhile :; do printf '0123456789012345678901234567890123456789'; done\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	start := time.Now()
	out := cliprobe.Run(ctx, script, dir, os.Environ(), nil)
	if time.Since(start) > 3*time.Second || len(out.Stdout) > 65536 || out.Failure != "timeout_or_cancelled" {
		t.Fatal("unbounded probe", len(out.Stdout), out.Failure)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\ni=0; while [ $i -lt 3000 ]; do printf '0123456789012345678901234567890123456789'; i=$((i+1)); done\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	out = cliprobe.Run(context.Background(), script, dir, os.Environ(), nil)
	if out.Failure != "output_limit" || len(out.Stdout) != 65536 {
		t.Fatal(fmt.Sprint("output bound failed: ", len(out.Stdout), out.Failure))
	}
}

// A Windows Cursor version folder is checked through its node.exe and
// index.js (D-052).
func TestCheckCLICursorNode(t *testing.T) {
	_, profile := checkFixture(t)
	dir := filepath.Join(t.TempDir(), "versions", "2026.11.05-86047df")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"node.exe", "index.js"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	node, _ := filepath.EvalSymlinks(filepath.Join(dir, "node.exe"))
	var seen [][]string
	r, err := checkCLI(context.Background(), "cursor", node, profile, "windows", func(_ context.Context, cli, _ string, _ []string, args []string) cliprobe.Output {
		seen = append(seen, append([]string{cli}, args...))
		if args[1] == "--version" {
			return cliprobe.Output{Stdout: []byte("2026.11.05-86047df\n")}
		}
		return cliprobe.Output{Stdout: []byte(`{"isAuthenticated":true}`)}
	})
	index := filepath.Join(filepath.Dir(node), "index.js")
	if err != nil || len(seen) != 2 || seen[0][0] != node || seen[0][1] != index || seen[1][2] != "status" || r.Authentication.State != "authenticated" {
		t.Fatal(r, err, seen)
	}
}
