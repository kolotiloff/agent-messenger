package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The relay's tests start this test binary as the coordinator program; with
// relayHelperVariable set it acts as a small coordinator server that says
// which copy of the program file it is.
const relayHelperVariable = "AGENT_MESSENGER_RELAY_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(relayHelperVariable) != "" {
		relayHelper()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func relayHelper() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(`{"version":"test"}`)
		return
	}
	exe, _ := os.Executable()
	gen, _ := os.ReadFile(filepath.Join(filepath.Dir(exe), "generation"))
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var h rpcHead
		_ = json.Unmarshal(scanner.Bytes(), &h)
		if !hasID(h.ID) {
			continue
		}
		answer := func() {
			raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": h.ID, "result": map[string]any{
				"generation": string(gen), "chat": os.Getenv(CoordinatorChatVariable), "method": h.Method}})
			os.Stdout.Write(append(raw, '\n'))
		}
		if h.Method == "tools/call" {
			go func() { time.Sleep(700 * time.Millisecond); answer() }()
			continue
		}
		answer()
	}
}

type relayAnswer struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Result struct {
		Generation string `json:"generation"`
		Chat       string `json:"chat"`
	} `json:"result"`
}

// copyProgram puts a copy of the test binary at path, as an update would.
func copyProgram(t *testing.T, path, generation string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "generation"), []byte(generation), 0o600); err != nil {
		t.Fatal(err)
	}
	temp := path + ".new"
	if err := os.WriteFile(temp, raw, 0o700); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		// As the Windows update does: the running file is renamed away.
		_ = os.Rename(path, fmt.Sprintf("%s.old-%d", path, time.Now().UnixNano()))
	}
	if err := os.Rename(temp, path); err != nil {
		t.Fatal(err)
	}
}

func TestRelaySwitchesToTheUpdatedProgram(t *testing.T) {
	oldEvery, oldInit := relayCheckEvery, relayInitTimeout
	relayCheckEvery, relayInitTimeout = 50*time.Millisecond, 10*time.Second
	defer func() { relayCheckEvery, relayInitTimeout = oldEvery, oldInit }()
	t.Setenv(relayHelperVariable, "1")
	program := filepath.Join(t.TempDir(), "messenger")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	copyProgram(t, program, "1")

	toRelay, claudeIn := io.Pipe()
	claudeOut, fromRelay := io.Pipe()
	done := make(chan error, 1)
	go func() { done <- runCoordinatorRelay(program, []string{"coordinator-serve"}, toRelay, fromRelay) }()
	answers := make(chan relayAnswer, 16)
	go func() {
		scanner := bufio.NewScanner(claudeOut)
		for scanner.Scan() {
			var a relayAnswer
			_ = json.Unmarshal(scanner.Bytes(), &a)
			answers <- a
		}
		close(answers)
	}()
	send := func(line string) {
		if _, err := io.WriteString(claudeIn, line+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	next := func() relayAnswer {
		select {
		case a := <-answers:
			return a
		case <-time.After(20 * time.Second):
			t.Fatal("no answer from the relay")
		}
		return relayAnswer{}
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	first := next()
	if first.Result.Generation != "1" || !validChatID(first.Result.Chat) {
		t.Fatalf("first answer = %+v", first)
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	// A call that is still running when the update lands finishes in the old program.
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{}}`)
	copyProgram(t, program, "2")
	var gotCall, gotChanged bool
	for !gotCall || !gotChanged {
		a := next()
		switch {
		case a.Method == "notifications/tools/list_changed":
			gotChanged = true
		case string(a.ID) == "2":
			if a.Result.Generation != "1" {
				t.Fatalf("the running call moved: %+v", a)
			}
			gotCall = true
		default:
			t.Fatalf("unexpected line %+v", a)
		}
	}
	send(`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	if a := next(); string(a.ID) != "3" || a.Result.Generation != "2" || a.Result.Chat != first.Result.Chat {
		t.Fatalf("after the update: %+v (chat before %s)", a, first.Result.Chat)
	}
	if err := claudeIn.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "closed") {
			t.Fatal(err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the relay did not stop")
	}
}

func TestRelayKeepsTheOldProgramWhenTheNewOneIsBroken(t *testing.T) {
	oldEvery := relayCheckEvery
	relayCheckEvery = 50 * time.Millisecond
	defer func() { relayCheckEvery = oldEvery }()
	t.Setenv(relayHelperVariable, "1")
	program := filepath.Join(t.TempDir(), "messenger")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	copyProgram(t, program, "1")
	toRelay, claudeIn := io.Pipe()
	claudeOut, fromRelay := io.Pipe()
	go func() { _ = runCoordinatorRelay(program, []string{"coordinator-serve"}, toRelay, fromRelay) }()
	defer claudeIn.Close()
	reader := bufio.NewScanner(claudeOut)
	read := func() relayAnswer {
		line := make(chan []byte, 1)
		go func() {
			if reader.Scan() {
				line <- append([]byte(nil), reader.Bytes()...)
			}
		}()
		select {
		case l := <-line:
			var a relayAnswer
			_ = json.Unmarshal(l, &a)
			return a
		case <-time.After(20 * time.Second):
			t.Fatal("no answer")
		}
		return relayAnswer{}
	}
	io.WriteString(claudeIn, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n")
	read()
	io.WriteString(claudeIn, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")
	// A half-written update: not a program at all.
	if runtime.GOOS == "windows" {
		_ = os.Rename(program, program+".old")
	}
	if err := os.WriteFile(program+".tmp", []byte("not a program"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(program+".tmp", program); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	io.WriteString(claudeIn, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`+"\n")
	if a := read(); string(a.ID) != "2" || a.Result.Generation != "1" {
		t.Fatalf("answer = %+v", a)
	}
}
