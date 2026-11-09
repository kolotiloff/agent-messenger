package broker

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"time"
)

// Discovery on the local network (T-052). The joining computer broadcasts a
// fixed question on DiscoveryPort; a host answers with its port while it has
// a usable short invitation, and stays silent otherwise. The answer is not
// trusted: the code exchange checks the host.

// DiscoveryPort is the UDP port hosts listen on. Tests change it.
var DiscoveryPort = DefaultPort

const discoverQuestion = "agent-messenger discover 1"

type discoverAnswer struct {
	V      int    `json:"v"`
	Broker string `json:"broker"`
	Port   int    `json:"port"`
}

// serveDiscovery answers questions on conn until ctx ends; answer says what
// to send, if anything.
func serveDiscovery(ctx context.Context, conn net.PacketConn, answer func() (discoverAnswer, bool)) {
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	buf := make([]byte, 256)
	for {
		n, from, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		// Only this computer and private networks are answered, like the
		// broker's own connections: "local network" listens on every
		// interface.
		if string(buf[:n]) != discoverQuestion || !localPeer(from) {
			continue
		}
		if a, ok := answer(); ok {
			raw, _ := json.Marshal(a)
			_, _ = conn.WriteTo(raw, from)
		}
	}
}

// discoveryTargets are the addresses the question goes to: the general
// broadcast, every private network's own broadcast address and this
// computer itself.
func discoveryTargets() []string {
	port := strconv.Itoa(DiscoveryPort)
	out := []string{net.JoinHostPort("255.255.255.255", port), net.JoinHostPort("127.0.0.1", port)}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil || !ip.IsPrivate() || len(ipnet.Mask) != net.IPv4len {
			continue
		}
		b := make(net.IP, net.IPv4len)
		for i := range ip {
			b[i] = ip[i] | ^ipnet.Mask[i]
		}
		out = append(out, net.JoinHostPort(b.String(), port))
	}
	return out
}

// Discover asks the local network for hosts with a usable short invitation
// and returns their addresses (host:port), waiting up to wait for answers.
func Discover(ctx context.Context, wait time.Duration) ([]string, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	for _, target := range discoveryTargets() {
		if addr, err := net.ResolveUDPAddr("udp4", target); err == nil {
			_, _ = conn.WriteTo([]byte(discoverQuestion), addr)
		}
	}
	deadline := time.Now().Add(wait)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetReadDeadline(deadline)
	var found candidates
	buf := make([]byte, 512)
	for {
		n, from, err := conn.ReadFrom(buf)
		if err != nil {
			return found.out, nil
		}
		var a discoverAnswer
		if json.Unmarshal(buf[:n], &a) != nil {
			continue
		}
		if found.add(from, a) && len(found.out) >= maxCandidates {
			return found.out, nil
		}
	}
}

// Discovery limits: answers are not trusted, so one address may name only
// a few brokers, and the list stays short enough to try in turn.
const (
	maxCandidates        = 8
	maxCandidatesPerAddr = 2
)

// candidates collects the hosts that answered, in order. A host is kept per
// (broker ID, address): anyone on the network can answer with the real
// host's broker ID, so keeping only the first answer per broker would let a
// faster wrong answer hide the real host. Each candidate proves the code in
// its own exchange (pairCandidates).
type candidates struct {
	out   []string
	seen  map[string]bool
	perIP map[string]int
}

func (c *candidates) add(from net.Addr, a discoverAnswer) bool {
	udp, ok := from.(*net.UDPAddr)
	if !ok || !localPeer(from) || a.V != 1 || !brokerID.MatchString(a.Broker) || !validPort(a.Port) || len(c.out) >= maxCandidates {
		return false
	}
	if c.seen == nil {
		c.seen, c.perIP = map[string]bool{}, map[string]int{}
	}
	ip := udp.IP.String()
	address := net.JoinHostPort(ip, strconv.Itoa(a.Port))
	key := a.Broker + "|" + address
	if c.seen[key] || c.perIP[ip] >= maxCandidatesPerAddr {
		return false
	}
	c.seen[key] = true
	c.perIP[ip]++
	c.out = append(c.out, address)
	return true
}
