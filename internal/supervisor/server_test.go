package supervisor

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/broker"
	"github.com/kolotiloff/agent-messenger/internal/control"
)

func serverCall(t *testing.T, root, action string, payload any) (broker.Status, control.Response) {
	t.Helper()
	text := ""
	if payload != nil {
		raw, _ := json.Marshal(payload)
		text = string(raw)
	}
	response, err := control.Call(root, control.Request{Cmd: "server", Action: action, Text: text})
	if err != nil {
		t.Fatal(err)
	}
	var st broker.Status
	if response.OK && action != "invite" {
		if err := json.Unmarshal(response.Overview, &st); err != nil {
			t.Fatal(err)
		}
	}
	return st, response
}

func TestServerCommandRunsTheHostedBroker(t *testing.T) {
	isolateUser(t)
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, root, DisabledLauncher{}) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-errc:
		case <-time.After(3 * time.Second):
		}
	})
	waitSocket(t, root)

	st, _ := serverCall(t, root, "status", nil)
	if st.Host.Enabled || st.Host.Running || st.Member.Connected || st.Exchange {
		t.Fatalf("server on by default %+v", st)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	st, response := serverCall(t, root, "enable", map[string]any{"scope": "local", "port": port})
	if !response.OK || !st.Host.Running || len(st.Host.Addresses) != 1 {
		t.Fatalf("enable %+v %+v", response, st)
	}
	_, response = serverCall(t, root, "invite", map[string]any{"address": st.Host.Addresses[0]})
	var code broker.InviteCode
	if !response.OK || json.Unmarshal(response.Overview, &code) != nil || !strings.HasPrefix(code.Code, broker.InvitePrefix) {
		t.Fatalf("invite %+v", response)
	}
	// This client joins its own broker, as a second computer would.
	st, response = serverCall(t, root, "connect", map[string]any{"code": code.Code, "name": "Этот Mac"})
	if !response.OK || !st.Member.Connected || !st.Member.Reachable || len(st.Host.Devices) != 1 {
		t.Fatalf("connect %+v %+v", response, st)
	}
	if status, err := control.Call(root, control.Request{Cmd: "status"}); err != nil || status.Status == nil || status.Status.Broker != "hosting+connected" {
		t.Fatalf("status %+v %v", status, err)
	}
	if strings.Contains(string(response.Overview), "device_key") {
		t.Fatal("device key in status")
	}
	st, _ = serverCall(t, root, "revoke", map[string]any{"device": st.Host.Devices[0].ID})
	st, response = serverCall(t, root, "check", nil)
	if !response.OK || st.Member.Reachable || st.Member.Error == "" {
		t.Fatalf("check after revoke %+v %+v", response, st)
	}
	if _, response = serverCall(t, root, "nonsense", nil); response.OK {
		t.Fatal("unknown action accepted")
	}
	st, _ = serverCall(t, root, "disconnect", nil)
	st, _ = serverCall(t, root, "disable", nil)
	if st.Host.Running || st.Member.Connected {
		t.Fatalf("after disable %+v", st)
	}
}
