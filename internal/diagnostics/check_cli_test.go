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
		output                      probeOutput
	}{
		{"claude", "2.1.318 (Claude Code)", "CLAUDE_CONFIG_DIR", []string{"auth", "status", "--json"}, probeOutput{stdout: []byte(`{"loggedIn":true,"email":"PRIVATE_EMAIL","orgId":"PRIVATE_ORG"}`)}},
		{"codex", "codex-cli 0.155.1", "CODEX_HOME", []string{"login", "status"}, probeOutput{stderr: []byte("Logged in using ChatGPT\n")}},
		{"cursor", "2026.11.05-86047df", "CURSOR_CONFIG_DIR", []string{"status", "--format", "json"}, probeOutput{stdout: []byte(`{"isAuthenticated":true,"userInfo":{"email":"PRIVATE_EMAIL"}}`)}},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			cli, profile := checkFixture(t)
			t.Setenv(tc.variable, "/wrong/inherited/profile")
			calls := 0
			var cwd string
			runner := func(ctx context.Context, path, dir string, env, args []string) probeOutput {
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
					return probeOutput{stdout: []byte(tc.version)}
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
	for _, version := range []probeOutput{
		{stdout: []byte("PRIVATE_TOKEN")},
		{stdout: []byte("2.1.318 (Claude Code)"), exit: 1},
		{stdout: []byte("2.1.318 (Claude Code)"), failure: "output_limit"},
		{failure: "timeout_or_cancelled"},
	} {
		calls := 0
		r, err := checkCLI(context.Background(), "claude", cli, profile, "darwin", func(context.Context, string, string, []string, []string) probeOutput { calls++; return version })
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
	runner := func(context.Context, string, string, []string, []string) probeOutput {
		t.Fatal("should not run CLI")
		return probeOutput{}
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
		r, err := checkCLI(context.Background(), "claude", cli, profile, goos, func(_ context.Context, _ string, _ string, _ []string, args []string) probeOutput {
			calls = append(calls, args)
			if args[0] == "--version" {
				return probeOutput{stdout: []byte("2.1.999 (Claude Code)\n")}
			}
			return probeOutput{stdout: []byte(`{"loggedIn":true}`)}
		})
		if err != nil || len(calls) != 2 || r.Version.Value != "2.1.999" || r.Authentication.State != "authenticated" || r.OS != goos {
			t.Fatal(goos, r, err, calls)
		}
	}
}

func TestAuthenticationRequiresPositiveEvidenceAndNeverLeaksSecrets(t *testing.T) {
	for _, tc := range []struct {
		provider string
		output   probeOutput
		state    string
	}{
		{"claude", probeOutput{stdout: []byte(`{"loggedIn":false}`), exit: 1}, "not_authenticated"},
		{"claude", probeOutput{stdout: []byte(`{"loggedIn":true}`), exit: 1}, "unknown"},
		{"claude", probeOutput{stdout: []byte(`{}`)}, "unknown"},
		{"claude", probeOutput{stdout: []byte(`{"loggedIn":"true"}`)}, "unknown"},
		{"claude", probeOutput{stdout: []byte(`{"loggedIn":null}`)}, "unknown"},
		{"claude", probeOutput{stdout: []byte(`{"loggedIn":true}`), failure: "timeout_or_cancelled"}, "unknown"},
		{"cursor", probeOutput{stdout: []byte(`{"isAuthenticated":false}`), exit: 1}, "unknown"},
		{"codex", probeOutput{stderr: []byte("Logged in using an API key - PRIVATE_KEY")}, "authenticated"},
		{"codex", probeOutput{stderr: []byte("Logged in using ChatGPT\nPRIVATE_WARNING")}, "unknown"},
		{"codex", probeOutput{stderr: []byte("Not logged in"), exit: 1}, "not_authenticated"},
		{"codex", probeOutput{stderr: []byte("PRIVATE_NETWORK_ERROR"), exit: 1}, "unknown"},
	} {
		r := parseAuthentication(tc.provider, tc.output)
		if r.State != tc.state {
			t.Fatal(tc.provider, r, tc.state)
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
	r, err := checkCLI(context.Background(), "claude", cli, profile, "darwin", func(context.Context, string, string, []string, []string) probeOutput {
		calls++
		if err := os.WriteFile(cli, []byte("changed size and content"), 0o700); err != nil {
			t.Fatal(err)
		}
		return probeOutput{stdout: []byte("2.1.318 (Claude Code)")}
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
	out := runProbe(ctx, script, dir, os.Environ(), nil)
	if time.Since(start) > 3*time.Second || len(out.stdout) > 65536 || out.failure != "timeout_or_cancelled" {
		t.Fatal("unbounded probe", len(out.stdout), out.failure)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\ni=0; while [ $i -lt 3000 ]; do printf '0123456789012345678901234567890123456789'; i=$((i+1)); done\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	out = runProbe(context.Background(), script, dir, os.Environ(), nil)
	if out.failure != "output_limit" || len(out.stdout) != 65536 {
		t.Fatal(fmt.Sprint("output bound failed: ", len(out.stdout), out.failure))
	}
}
