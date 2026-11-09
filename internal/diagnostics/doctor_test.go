package diagnostics

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
)

func TestDoctorDiscoversCandidatesWithoutExecutingOrCreatingData(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable fixtures")
	}
	dir := t.TempDir()
	canary := filepath.Join(dir, "cli-was-started")
	t.Setenv("MESSENGER_DOCTOR_CANARY", canary)
	t.Setenv("PATH", dir)
	for _, name := range []string{"claude", "codex", "agent"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\necho called >> \"$MESSENGER_DOCTOR_CANARY\"\nexit 99\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(dir, "does-not-exist")
	r := Collect(context.Background(), root)
	if r.SchemaVersion != 1 || r.Data.State != "missing" || r.Controller.State != "unreachable" || r.ProbeCLIStarts != 0 {
		t.Fatal(r)
	}
	if len(r.Agents) != 3 {
		t.Fatal(r.Agents)
	}
	for _, a := range r.Agents {
		if a.CLI.State != "candidate_found" || a.Authentication != "not_checked" || a.CLI.Version != "not_checked" || a.CLI.Identity != "not_checked" || a.Session != "not_checked" {
			t.Fatal("file discovery must not imply readiness", a)
		}
	}
	if r.Agents[2].CLI.Command != "agent" {
		t.Fatal("Cursor fallback not identified as a candidate")
	}
	if _, err := os.Stat(canary); !os.IsNotExist(err) {
		t.Fatal("doctor executed a candidate", err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("doctor created its data root", err)
	}
}

func TestDoctorMissingExecutableIsNotMissingAuthentication(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	r := Collect(context.Background(), filepath.Join(dir, "missing"))
	for _, a := range r.Agents {
		if a.CLI.State != "not_found" || a.Authentication != "not_checked" {
			t.Fatal(a)
		}
	}
	if _, err := time.Parse(time.RFC3339Nano, r.CheckedAt); err != nil {
		t.Fatal(err)
	}
}

func shortRoot(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("native Unix socket fixture")
	}
	root, err := os.MkdirTemp("", "am-doc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	return root
}

func TestDoctorReadsOnlyStatusAndOmitsPrivateDetails(t *testing.T) {
	root := shortRoot(t)
	ln, err := net.Listen("unix", control.SocketPath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	requests := make(chan control.Request, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var request control.Request
		json.NewDecoder(conn).Decode(&request)
		requests <- request
		json.NewEncoder(conn).Encode(control.Response{OK: true, Status: &control.Status{Supervisor: 123,
			Paused: true, CLILaunch: "tasks", TasksEnabled: true, Broker: "not_configured", Detail: "PRIVATE_FIXTURE_DETAIL",
			Reservations: &control.Reservations{Old: []control.OldReservation{{Kind: "session", Root: "/PRIVATE_FIXTURE_ROOT"}}}},
			Overview: json.RawMessage(`{"private":"PRIVATE_FIXTURE_INSPECT"}`)})
	}()
	r := Collect(context.Background(), root)
	if r.Controller.State != "reachable" || r.Controller.Status == nil || !r.Controller.Status.TasksEnabled || !r.Controller.Status.Paused || r.Controller.Status.Reservations != nil {
		t.Fatal(r.Controller)
	}
	select {
	case req := <-requests:
		if req.Cmd != "status" || req.Text != "" || req.ConversationID != "" {
			t.Fatal("diagnostic requested execution data", req)
		}
	case <-time.After(time.Second):
		t.Fatal("no status request")
	}
	encoded, _ := json.Marshal(r)
	if strings.Contains(string(encoded), "PRIVATE_FIXTURE") {
		t.Fatal("private status or inspect data leaked")
	}
}

func TestDoctorUnresponsiveSocketReturnsWithoutUnlocking(t *testing.T) {
	root := shortRoot(t)
	reserve := filepath.Join(root, "active-runs.json")
	content := []byte("private reservation fixture")
	os.WriteFile(reserve, content, 0o600)
	ln, err := net.Listen("unix", control.SocketPath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	closed := make(chan struct{})
	defer close(closed)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		<-closed
	}()
	start := time.Now()
	r := Collect(context.Background(), root)
	if time.Since(start) > 3*time.Second || r.Controller.State != "unreachable" || r.Controller.Status != nil {
		t.Fatal("unresponsive controller misreported or hung", r.Controller)
	}
	after, err := os.ReadFile(reserve)
	if err != nil || string(after) != string(content) {
		t.Fatal("diagnosis changed reservation", err)
	}
}

func TestDoctorRejectsFailedStatusAsReadiness(t *testing.T) {
	root := shortRoot(t)
	ln, err := net.Listen("unix", control.SocketPath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var req control.Request
		json.NewDecoder(conn).Decode(&req)
		json.NewEncoder(conn).Encode(control.Response{OK: false, Error: "PRIVATE_FAILURE", Status: &control.Status{Supervisor: 123}})
	}()
	r := Collect(context.Background(), root)
	if r.Controller.State != "unknown" || r.Controller.Status != nil {
		t.Fatal(r.Controller)
	}
	encoded, _ := json.Marshal(r)
	if strings.Contains(string(encoded), "PRIVATE_FAILURE") {
		t.Fatal("raw error leaked")
	}
}
