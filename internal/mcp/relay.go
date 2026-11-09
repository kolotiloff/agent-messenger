package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// Coordinator relay (T-117, D-107). Claude starts `messenger coordinator`
// once per chat and keeps that process for the chat's life. So that an
// updated program serves the same chat, that process is a small relay: it
// runs `coordinator-serve` from the program file and passes MCP lines both
// ways. When the program file changes (an update), the relay starts the new
// program, initializes it with the chat's own initialize request, sends new
// calls there and tells Claude the tool list changed. Calls already running
// in the old program finish there; it is closed when none is left. A new
// program that fails to start or answer is dropped and the old one keeps
// serving; the relay tries again only after the file changes again.

// CoordinatorChatVariable passes the relay's chat id to each program it
// starts, so a switch is not a new chat (T-090).
const CoordinatorChatVariable = "AGENT_MESSENGER_COORDINATOR_CHAT"

// CoordinatorServeCommand is the subcommand the relay starts.
const CoordinatorServeCommand = "coordinator-serve"

// relayCheckEvery is how often the relay looks at the program file.
var relayCheckEvery = 30 * time.Second

// relayInitTimeout bounds a new program's answer to initialize.
var relayInitTimeout = 30 * time.Second

func validChatID(id string) bool {
	if !strings.HasPrefix(id, "chat-") || len(id) != len("chat-")+16 {
		return false
	}
	for _, r := range id[len("chat-"):] {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

type relayChild struct {
	cmd     *exec.Cmd
	in      io.WriteCloser
	pending map[string]bool
	closing bool
	init    chan json.RawMessage
	done    chan struct{}
}

type coordinatorRelay struct {
	program string
	args    []string
	chat    string
	out     io.Writer
	outMu   sync.Mutex

	mu      sync.Mutex
	current *relayChild
	alive   []*relayChild
	initReq []byte // the chat's initialize request, as Claude sent it
	initID  json.RawMessage
	ready   bool // Claude finished its handshake
	seq     int
	last    os.FileInfo
	failed  chan struct{} // closed when the current program exits by itself
	stopped bool
}

// CoordinatorRelayMain runs the relay for program (the file Claude starts);
// args are the full arguments of its coordinator-serve command.
func CoordinatorRelayMain(program string, args []string) error {
	return runCoordinatorRelay(program, args, os.Stdin, os.Stdout)
}

func runCoordinatorRelay(program string, args []string, in io.Reader, out io.Writer) error {
	chat, err := core.NewChatID()
	if err != nil {
		return err
	}
	r := &coordinatorRelay{program: program, args: args, chat: chat, out: out, failed: make(chan struct{})}
	r.last, _ = os.Stat(program)
	if _, err := r.start(); err != nil {
		return err
	}
	stop := make(chan struct{})
	defer close(stop)
	go r.watch(stop)
	lines := make(chan []byte)
	readErr := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			lines <- append([]byte(nil), scanner.Bytes()...)
		}
		readErr <- scanner.Err()
		close(lines)
	}()
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				r.shutdown()
				return <-readErr
			}
			r.fromClaude(line)
		case <-r.failed:
			r.shutdown()
			return errors.New("the coordinator program stopped")
		}
	}
}

type rpcHead struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func hasID(id json.RawMessage) bool { return len(id) > 0 && string(id) != "null" }

// fromClaude sends one line from Claude to the program that should get it.
func (r *coordinatorRelay) fromClaude(line []byte) {
	var h rpcHead
	_ = json.Unmarshal(line, &h)
	r.mu.Lock()
	defer r.mu.Unlock()
	target := r.current
	switch {
	case h.Method == "initialize" && hasID(h.ID):
		r.initReq, r.initID = append([]byte(nil), line...), h.ID
	case h.Method == "notifications/initialized":
		r.ready = true
	case h.Method == "notifications/cancelled":
		var p struct {
			RequestID json.RawMessage `json:"requestId"`
		}
		_ = json.Unmarshal(h.Params, &p)
		for _, c := range r.alive {
			if c.pending[string(p.RequestID)] {
				target = c
			}
		}
	}
	if h.Method != "" && hasID(h.ID) {
		target.pending[string(h.ID)] = true
	}
	_, _ = target.in.Write(append(line, '\n'))
}

func (r *coordinatorRelay) toClaude(line []byte) {
	r.outMu.Lock()
	defer r.outMu.Unlock()
	_, _ = r.out.Write(append(line, '\n'))
}

// start runs the program file's coordinator server and passes its output on.
func (r *coordinatorRelay) start() (*relayChild, error) {
	cmd := exec.Command(r.program, r.args...)
	cmd.Env = append(os.Environ(), CoordinatorChatVariable+"="+r.chat, CoordinatorProgramVariable+"="+r.program)
	cmd.Stderr = os.Stderr
	platform.HideConsole(cmd)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &relayChild{cmd: cmd, in: in, pending: map[string]bool{}, init: make(chan json.RawMessage, 1), done: make(chan struct{})}
	r.mu.Lock()
	r.alive = append(r.alive, c)
	if r.current == nil {
		r.current = c
	}
	r.mu.Unlock()
	go r.read(c, stdout)
	return c, nil
}

// read passes a program's lines to Claude, except the answer to the relay's
// own initialize of a new program.
func (r *coordinatorRelay) read(c *relayChild, stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var h rpcHead
		_ = json.Unmarshal(line, &h)
		if h.Method == "" && hasID(h.ID) && strings.HasPrefix(string(h.ID), `"am-relay-`) {
			select {
			case c.init <- append(json.RawMessage(nil), line...):
			default:
			}
			continue
		}
		r.mu.Lock()
		current := c == r.current
		if h.Method == "" && hasID(h.ID) {
			delete(c.pending, string(h.ID))
		}
		r.mu.Unlock()
		if h.Method != "" && !current {
			continue // only the current program speaks for itself
		}
		r.toClaude(line)
		r.mu.Lock()
		if c.closing && len(c.pending) == 0 {
			_ = c.in.Close()
		}
		r.mu.Unlock()
	}
	_ = c.cmd.Wait()
	close(c.done)
	r.mu.Lock()
	for i, a := range r.alive {
		if a == c {
			r.alive = append(r.alive[:i], r.alive[i+1:]...)
			break
		}
	}
	// A current program that exits by itself ends the relay; Claude then
	// shows the server as stopped and can start it again.
	gone := c == r.current && !r.stopped
	r.mu.Unlock()
	if gone {
		close(r.failed)
	}
}

// watch looks at the program file and switches after it changed.
func (r *coordinatorRelay) watch(stop <-chan struct{}) {
	tick := time.NewTicker(relayCheckEvery)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			r.check()
		}
	}
}

func (r *coordinatorRelay) check() {
	now, err := os.Stat(r.program)
	if err != nil || !now.Mode().IsRegular() {
		return
	}
	r.mu.Lock()
	changed := r.last == nil || !os.SameFile(r.last, now) || !r.last.ModTime().Equal(now.ModTime()) || r.last.Size() != now.Size()
	ready := r.ready && r.initReq != nil
	r.mu.Unlock()
	if !changed || !ready {
		return
	}
	// The file is taken as seen whatever happens next: a program that fails
	// is not tried again until the file changes again.
	r.mu.Lock()
	r.last = now
	r.mu.Unlock()
	if err := r.switchProgram(); err != nil {
		fmt.Fprintln(os.Stderr, "messenger coordinator: the updated program was not taken:", err)
	}
}

// switchProgram starts the program file again and moves new calls to it.
func (r *coordinatorRelay) switchProgram() error {
	if !programAnswers(r.program) {
		return errors.New("it does not answer `version`")
	}
	c, err := r.start()
	if err != nil {
		return err
	}
	drop := func() {
		_ = c.in.Close()
		_ = c.cmd.Process.Kill()
	}
	r.mu.Lock()
	r.seq++
	id, _ := json.Marshal(fmt.Sprintf("am-relay-%d", r.seq))
	var req map[string]json.RawMessage
	err = json.Unmarshal(r.initReq, &req)
	r.mu.Unlock()
	if err != nil {
		drop()
		return err
	}
	req["id"] = id
	line, _ := json.Marshal(req)
	if _, err := c.in.Write(append(line, '\n')); err != nil {
		drop()
		return err
	}
	select {
	case answer := <-c.init:
		var a struct {
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal(answer, &a) != nil || hasID(a.Error) {
			drop()
			return errors.New("it refused initialize")
		}
	case <-c.done:
		return errors.New("it stopped at start")
	case <-time.After(relayInitTimeout):
		drop()
		return errors.New("it did not answer initialize")
	}
	if _, err := c.in.Write([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")); err != nil {
		drop()
		return err
	}
	r.mu.Lock()
	old := r.current
	r.current = c
	old.closing = true
	if len(old.pending) == 0 {
		_ = old.in.Close()
	}
	r.mu.Unlock()
	r.toClaude([]byte(`{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`))
	return nil
}

// programAnswers runs `program version`, so a half-written or broken file
// is never started as the coordinator.
func programAnswers(program string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, "version")
	platform.HideConsole(cmd)
	out, err := cmd.Output()
	return err == nil && bytes.Contains(out, []byte(`"version"`))
}

func (r *coordinatorRelay) shutdown() {
	r.mu.Lock()
	r.stopped = true
	alive := append([]*relayChild(nil), r.alive...)
	r.mu.Unlock()
	for _, c := range alive {
		_ = c.in.Close()
	}
	for _, c := range alive {
		select {
		case <-c.done:
		case <-time.After(10 * time.Second):
			_ = c.cmd.Process.Kill()
			<-c.done
		}
	}
}
