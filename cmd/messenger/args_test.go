package main

import "testing"

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
