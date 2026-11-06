package main

import (
	"strings"
	"testing"
)

func TestTimelineRejectsInvalidArgumentsBeforeConnecting(t *testing.T) {
	for _, args := range [][]string{{"timeline"}, {"timeline", "--alias", " "}, {"timeline", "--alias", "worker", "--limit", "0"}, {"timeline", "--alias", "worker", "--limit", "101"}, {"timeline", "--alias", "worker", "extra"}} {
		err := run(t.TempDir(), args)
		if err == nil || !strings.Contains(err.Error(), "timeline requires") {
			t.Fatalf("bad arguments reached socket: %v", err)
		}
	}
}

func TestServeModelFlagIsExplicit(t *testing.T) {
	allow, err := serveAllowsModels(nil)
	if err != nil || allow {
		t.Fatalf("default = %t %v", allow, err)
	}
	allow, err = serveAllowsModels([]string{"--allow-model-calls"})
	if err != nil || !allow {
		t.Fatalf("opt-in = %t %v", allow, err)
	}
	if _, err := serveAllowsModels([]string{"--root"}); err == nil {
		t.Fatal("unknown serve flag accepted")
	}
	allow, err = serveAllowsModels([]string{"--claude-stream"})
	if err != nil || allow {
		t.Fatal("stream flag authorized model calls", allow, err)
	}
	allow, err = serveAllowsModels([]string{"--claude-stream", "--allow-model-calls"})
	if err != nil || !allow {
		t.Fatal("stream opt-in rejected", err)
	}
}

func TestRootFlagWorksBeforeOrAfterCommand(t *testing.T) {
	root, args := splitArgs([]string{"--root", "/tmp/data", "serve"})
	if root != "/tmp/data" || len(args) != 1 || args[0] != "serve" {
		t.Fatalf("before = %q %q", root, args)
	}
	root, args = splitArgs([]string{"serve", "--root", "/tmp/data"})
	if root != "/tmp/data" || len(args) != 1 || args[0] != "serve" {
		t.Fatalf("after = %q %q", root, args)
	}
	root, args = splitArgs([]string{"status", "--root", "/tmp/other"})
	if root != "/tmp/other" || args[0] != "status" {
		t.Fatalf("status = %q %q", root, args)
	}
}

func TestScopedSessionNeverGrantsModelPermission(t *testing.T) {
	for _, args := range [][]string{{"--claude-stream", "--only-session=worker"}, {"--only-session", "worker", "--claude-stream"}} {
		o, err := serveOptions(args)
		if err != nil || o.AllowModels || !o.ClaudeStream || o.OnlySession != "worker" {
			t.Fatalf("scope options = %+v, %v", o, err)
		}
		o, err = serveOptions(append(args, "--allow-model-calls"))
		if err != nil || !o.AllowModels || o.OnlySession != "worker" {
			t.Fatalf("explicit scope permission = %+v, %v", o, err)
		}
	}
	for _, args := range [][]string{
		{"--claude-stream", "--only-session"},
		{"--claude-stream", "--only-session="},
		{"--claude-stream", "--only-session", "--allow-model-calls"},
		{"--claude-stream", "--only-session=one", "--only-session=two"},
		{"--claude-stream", "--only-session= one "},
		{"--claude-stream", "--only-session=one\ntwo"},
		{"--only-session=worker", "--allow-model-calls"},
	} {
		if _, err := serveOptions(args); err == nil {
			t.Fatalf("unsafe scope accepted: %q", args)
		}
	}
}

func TestRecoverStreamRejectsMissingBindingAndBypassFlags(t *testing.T) {
	for _, args := range [][]string{
		{"recover-stream"},
		{"recover-stream", "--confirm", "preserve-uncertain"},
		{"recover-stream", "--alias", "worker", "--confirm", "preserve-uncertain"},
		{"recover-stream", "--run", "run", "--confirm", "preserve-uncertain"},
		{"recover-stream", "--force"},
		{"recover-stream", "unexpected"},
	} {
		if err := run(t.TempDir(), args); err == nil {
			t.Fatal("invalid recovery command accepted", args)
		}
	}
}
