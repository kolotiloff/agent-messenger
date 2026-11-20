package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/broker"
)

func TestAgentVersionOnAnotherComputer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX CLI fixture")
	}
	ctx := context.Background()
	host, _ := netComputer(t, ctx, "a")
	worker, _ := netComputer(t, ctx, "b")
	if err := os.WriteFile(filepath.Join(worker.root, "codex"), []byte("#!/bin/sh\n[ \"$#\" -eq 1 ] && [ \"$1\" = '--version' ] || exit 9\necho 'codex-cli 0.155.1'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := host.server.Enable(broker.ScopeLocal, freeLocalPort(t)); err != nil {
		t.Fatal(err)
	}
	invite, err := host.server.Invite(host.server.Status().Host.Addresses[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.server.Connect(ctx, invite.Code, "ПК-2"); err != nil {
		t.Fatal(err)
	}
	worker.server.ReportPresence(ctx)
	host.networkRound(ctx)
	peers, err := host.store.NetPeers()
	if err != nil || len(peers) != 1 {
		t.Fatalf("peers %+v: %v", peers, err)
	}
	id, err := host.store.RequestAgentVersion(peers[0].ID, "codex")
	if err != nil {
		t.Fatal(err)
	}
	host.networkRound(ctx)
	worker.networkRound(ctx)
	net, ok := worker.server.Network()
	if !ok {
		t.Fatal("worker left the network")
	}
	worker.answerAgentVersion(ctx, net.BrokerID)
	worker.networkRound(ctx)
	host.networkRound(ctx)
	v, err := host.store.AgentVersion(id)
	if err != nil || v.Computer != "ПК-2" || v.Agent != "codex" || v.State != "reported" || v.Version != "0.155.1" || v.CheckedAt == "" {
		t.Fatalf("remote version %+v: %v", v, err)
	}
}
