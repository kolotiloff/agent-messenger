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
	// The server is off by default; accepting the network's tasks is on (D-056).
	if st.Host.Enabled || st.Host.Running || st.Member.Connected || !st.Accept {
		t.Fatalf("defaults %+v", st)
	}
	if st, response := serverCall(t, root, "accept", map[string]any{"on": true}); !response.OK || !st.Accept {
		t.Fatalf("accept on %+v %+v", response, st)
	}
	if st, _ := serverCall(t, root, "accept", map[string]any{"on": false}); st.Accept {
		t.Fatal("accept stays on")
	}
	// The owner names this computer for the network.
	if _, response := serverCall(t, root, "rename", map[string]any{"name": "Студия"}); !response.OK {
		t.Fatalf("rename %+v", response)
	}
	if _, response := serverCall(t, root, "rename", map[string]any{"name": "a\nb"}); response.OK {
		t.Fatal("a name with a line break was accepted")
	}
	port := freeTCPPort(t)
	st, response := serverCall(t, root, "enable", map[string]any{"scope": "local", "port": port})
	if !response.OK || !st.Host.Running || len(st.Host.Addresses) != 1 {
		t.Fatalf("enable %+v %+v", response, st)
	}
	_, response = serverCall(t, root, "invite", map[string]any{"address": st.Host.Addresses[0]})
	var code broker.InviteCode
	if !response.OK || json.Unmarshal(response.Overview, &code) != nil || !strings.HasPrefix(code.Code, broker.InvitePrefix) {
		t.Fatalf("invite %+v", response)
	}
	// A computer hosts its network or joins another one, not both (D-111).
	if _, response = serverCall(t, root, "connect", map[string]any{"code": code.Code, "name": "Этот Mac"}); response.OK || !strings.Contains(response.Error, "остановите") {
		t.Fatalf("joined while hosting %+v", response)
	}
	if st, _ = serverCall(t, root, "disable", nil); st.Host.Running {
		t.Fatalf("after disable %+v", st)
	}
	other := broker.NewService(context.Background(), t.TempDir())
	t.Cleanup(other.Close)
	if err := other.Enable(broker.ScopeLocal, freeTCPPort(t)); err != nil {
		t.Fatal(err)
	}
	invite, err := other.Invite(other.Status().Host.Addresses[0])
	if err != nil {
		t.Fatal(err)
	}
	st, response = serverCall(t, root, "connect", map[string]any{"code": invite.Code, "name": "Этот Mac"})
	if !response.OK || !st.Member.Connected || !st.Member.Reachable {
		t.Fatalf("connect %+v %+v", response, st)
	}
	if status, err := control.Call(root, control.Request{Cmd: "status"}); err != nil || status.Status == nil || status.Status.Broker != "connected" {
		t.Fatalf("status %+v %v", status, err)
	}
	if strings.Contains(string(response.Overview), "device_key") {
		t.Fatal("device key in status")
	}
	if _, response = serverCall(t, root, "enable", map[string]any{"scope": "local", "port": port}); response.OK || !strings.Contains(response.Error, "отключите") {
		t.Fatalf("hosted while joined %+v", response)
	}
	if err := other.Revoke(other.Status().Host.Devices[0].ID); err != nil {
		t.Fatal(err)
	}
	st, response = serverCall(t, root, "check", nil)
	if !response.OK || st.Member.Reachable || st.Member.Error == "" {
		t.Fatalf("check after revoke %+v %+v", response, st)
	}
	if _, response = serverCall(t, root, "nonsense", nil); response.OK {
		t.Fatal("unknown action accepted")
	}
	if st, _ = serverCall(t, root, "disconnect", nil); st.Host.Running || st.Member.Connected {
		t.Fatalf("after disconnect %+v", st)
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
