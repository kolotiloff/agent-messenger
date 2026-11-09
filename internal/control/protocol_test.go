package control

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
	"sync/atomic"
	"testing"
)

type statusHandler struct {
	Handler // Other calls panic.
	calls   *atomic.Int32
}

func (h statusHandler) Status() (Status, error) {
	h.calls.Add(1)
	return Status{Broker: "ok"}, nil
}

func TestServeBoundsTheRequestLine(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	var calls atomic.Int32
	go func() { _ = Serve(ln, statusHandler{calls: &calls}) }()
	send := func(line string) Response {
		t.Helper()
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		go func() { _, _ = conn.Write([]byte(line)) }()
		reply, err := bufio.NewReader(conn).ReadBytes('\n')
		if err != nil {
			t.Fatalf("no reply: %v", err)
		}
		var r Response
		if err := json.Unmarshal(reply, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	request := func(textLen int) string {
		encoded, _ := json.Marshal(Request{Cmd: "status", Text: strings.Repeat("я", textLen/2)})
		return string(encoded) + "\n"
	}
	// A large but legitimate request still works.
	if r := send(request(1 << 20)); !r.OK || r.Status == nil || calls.Load() != 1 {
		t.Fatalf("large request: %+v", r)
	}
	// A line longer than the limit is refused without being handled, with
	// or without its newline.
	for _, line := range []string{request(MaxRequestBytes), strings.Repeat("x", MaxRequestBytes+10)} {
		if r := send(line); r.OK || r.Error == "" || calls.Load() != 1 {
			t.Fatalf("oversized request: %+v calls %d", r, calls.Load())
		}
	}
}
