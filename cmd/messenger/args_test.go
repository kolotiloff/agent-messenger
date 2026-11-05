package main

import "testing"

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
