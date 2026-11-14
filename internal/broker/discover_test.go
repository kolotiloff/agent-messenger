package broker

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestDiscoverySurvivesLostFirstDatagram(t *testing.T) {
	host, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	client, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 256)
		count := 0
		for {
			n, from, err := host.ReadFrom(buf)
			if err != nil {
				return
			}
			if string(buf[:n]) != discoverQuestion {
				continue
			}
			count++
			if count == 1 {
				continue
			}
			raw, _ := json.Marshal(discoverAnswer{V: 1, Broker: "brk-1111111111111111", Port: 47100})
			host.WriteTo(raw, from)
			return
		}
	}()
	got, err := discoverWithTargets(context.Background(), client, []string{host.LocalAddr().String()}, 900*time.Millisecond)
	if err != nil || len(got) != 1 || got[0] != "127.0.0.1:47100" {
		t.Fatal(got, err)
	}
	<-done
}
func TestDiscoveryCancellation(t *testing.T) {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, err = discoverWithTargets(ctx, conn, nil, time.Second)
	if err != context.Canceled || time.Since(started) > 500*time.Millisecond {
		t.Fatal(err)
	}
}
func TestUnavailableDiscoveryDoesNotOfferShortCode(t *testing.T) {
	occupied, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	old := DiscoveryPort
	DiscoveryPort = occupied.LocalAddr().(*net.UDPAddr).Port
	t.Cleanup(func() { DiscoveryPort = old })
	host := newService(t, t.TempDir())
	if err := host.Enable(ScopeLocal, freePort(t)); err != nil {
		t.Fatal(err)
	}
	status := host.Status()
	if status.Host.DiscoveryError == "" {
		t.Fatal("UDP failure hidden")
	}
	code, err := host.Invite(status.Host.Addresses[0])
	if err != nil {
		t.Fatal(err)
	}
	if code.Short != "" || code.ShortUnavailable == "" || code.Code == "" {
		t.Fatal("unusable short code offered")
	}
	// The long invitation still registers normally; a busy UDP port must not
	// disable the actual broker or consume an invitation.
	member := newService(t, t.TempDir())
	if err := member.Connect(ctx(t), code.Code, "fixture"); err != nil {
		t.Fatal(err)
	}
}
