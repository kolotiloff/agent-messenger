package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// lockedBuffer records what the client wrote.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := strings.TrimSpace(b.buf.String())
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func callWith(t *testing.T, server string, method string) (json.RawMessage, error, *lockedBuffer) {
	t.Helper()
	out := &lockedBuffer{}
	rpc := NewCodexRPC(out, strings.NewReader(server))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := rpc.Call(ctx, method, map[string]any{})
	return result, err, out
}

func TestCodexRPCSkipsNotificationsAndMatchesID(t *testing.T) {
	server := `{"method":"configWarning","params":{"summary":"x"}}` + "\n" +
		`{"method":"remoteControl/status/changed","params":{}}` + "\n" +
		`{"id":1,"result":{"userAgent":"fake"}}` + "\n"
	result, err, out := callWith(t, server, "initialize")
	if err != nil || !strings.Contains(string(result), "fake") {
		t.Fatalf("result %s err %v", result, err)
	}
	if lines := out.lines(); len(lines) != 1 || !strings.Contains(lines[0], `"method":"initialize"`) || !strings.Contains(lines[0], `"id":1`) {
		t.Fatalf("wrote %q", lines)
	}
}

func TestCodexRPCFailsClosedWithoutRetry(t *testing.T) {
	for name, server := range map[string]string{
		"server request":      `{"id":5,"method":"item/tool/call","params":{}}`,
		"wrong id":            `{"id":2,"result":{}}`,
		"string id":           `{"id":"1","result":{}}`,
		"error response":      `{"id":1,"error":{"code":-1,"message":"private-data"}}`,
		"result and error":    `{"id":1,"result":{},"error":{"code":-1,"message":"private-data"}}`,
		"null result":         `{"id":1,"result":null}`,
		"response without id": `{"result":{}}`,
		"malformed":           `{"id":1`,
		"eof":                 ``,
	} {
		t.Run(name, func(t *testing.T) {
			_, err, out := callWith(t, server+"\n", "config/read")
			if err == nil {
				t.Fatal("ambiguous response accepted")
			}
			if strings.Contains(err.Error(), "private-data") {
				t.Fatal("server error text leaked")
			}
			if n := len(out.lines()); n != 1 {
				t.Fatalf("request written %d times", n)
			}
		})
	}
}

func TestCodexRPCRejectsModelMethodsBeforeWriting(t *testing.T) {
	for _, method := range []string{"turn/start", "thread/resume", "turn/steer", ""} {
		_, err, out := callWith(t, "", method)
		if err == nil || len(out.lines()) != 0 {
			t.Fatalf("%q escaped the setup allowlist", method)
		}
	}
	out := &lockedBuffer{}
	rpc := NewCodexRPC(out, strings.NewReader(""))
	if rpc.Notify("turn/start") == nil || len(out.lines()) != 0 {
		t.Fatal("notification allowlist bypassed")
	}
	if err := rpc.Notify("initialized"); err != nil || len(out.lines()) != 1 {
		t.Fatalf("initialized not sent: %v", err)
	}
}

func TestCodexRPCTimeoutClosesClient(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	out := &lockedBuffer{}
	rpc := NewCodexRPC(out, reader)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := rpc.Call(ctx, "thread/start", map[string]any{}); err == nil {
		t.Fatal("timeout accepted")
	}
	// A late answer must not be paired with a new request; the client is closed.
	if _, err := rpc.Call(context.Background(), "thread/start", map[string]any{}); err == nil {
		t.Fatal("call after timeout accepted")
	}
	if n := len(out.lines()); n != 1 {
		t.Fatalf("thread/start written %d times", n)
	}
}

func TestCodexRPCDrainReportsServerRequests(t *testing.T) {
	server := `{"method":"thread/started","params":{}}` + "\n" + `{"id":7,"method":"item/tool/call","params":{}}` + "\n"
	rpc := NewCodexRPC(io.Discard, strings.NewReader(server))
	requests := 0
	if err := rpc.Drain(func() { requests++ }); err != io.EOF {
		t.Fatalf("drain ended with %v", err)
	}
	if requests != 1 {
		t.Fatalf("server requests seen: %d", requests)
	}
}

func TestCodexThreadStartParamsDeclareBoundTools(t *testing.T) {
	raw, err := json.Marshal(CodexThreadStartParams("/work"))
	if err != nil {
		t.Fatal(err)
	}
	var params struct {
		Cwd            string `json:"cwd"`
		Sandbox        string `json:"sandbox"`
		ApprovalPolicy string `json:"approvalPolicy"`
		Ephemeral      *bool  `json:"ephemeral"`
		DynamicTools   []struct {
			Type, Name string
			Tools      []struct{ Type, Name string }
		} `json:"dynamicTools"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatal(err)
	}
	if params.Cwd != "/work" || params.Sandbox != "read-only" || params.ApprovalPolicy != "never" || params.Ephemeral == nil || *params.Ephemeral {
		t.Fatalf("params %s", raw)
	}
	if len(params.DynamicTools) != 1 || params.DynamicTools[0].Type != "namespace" || params.DynamicTools[0].Name != "messenger_bound" {
		t.Fatalf("tools %s", raw)
	}
	var names []string
	for _, tool := range params.DynamicTools[0].Tools {
		if tool.Type != "function" {
			t.Fatalf("tool type %q", tool.Type)
		}
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "read_current,reply" {
		t.Fatalf("tool names %v", names)
	}
}

func threadStartResult(mutate func(map[string]any)) json.RawMessage {
	thread := map[string]any{
		"id": "01a1b0a0-1ef6-7861-8fd1-f7fb5912e2f9", "cwd": "/work", "ephemeral": false,
		"status": map[string]any{"type": "idle"}, "turns": []any{},
	}
	result := map[string]any{
		"approvalPolicy": "never", "sandbox": map[string]any{"type": "readOnly", "networkAccess": false},
		"cwd": "/work", "model": "m", "modelProvider": "p", "approvalsReviewer": "user", "thread": thread,
	}
	if mutate != nil {
		mutate(result)
	}
	raw, _ := json.Marshal(result)
	return raw
}

func TestParseCodexThreadStart(t *testing.T) {
	id, err := ParseCodexThreadStart(threadStartResult(nil), "/work")
	if err != nil || id != "01a1b0a0-1ef6-7861-8fd1-f7fb5912e2f9" {
		t.Fatalf("id %q err %v", id, err)
	}
	thread := func(key string, value any) func(map[string]any) {
		return func(r map[string]any) {
			if value == nil {
				delete(r["thread"].(map[string]any), key)
				return
			}
			r["thread"].(map[string]any)[key] = value
		}
	}
	for name, mutate := range map[string]func(map[string]any){
		"ephemeral":       thread("ephemeral", true),
		"no ephemeral":    thread("ephemeral", nil),
		"no id":           thread("id", nil),
		"bad id":          thread("id", "../etc"),
		"other cwd":       thread("cwd", "/elsewhere"),
		"active":          thread("status", map[string]any{"type": "active"}),
		"has turns":       thread("turns", []any{map[string]any{"id": "t"}}),
		"writable":        func(r map[string]any) { r["sandbox"] = map[string]any{"type": "workspaceWrite"} },
		"approvals":       func(r map[string]any) { r["approvalPolicy"] = "on-request" },
		"granular policy": func(r map[string]any) { r["approvalPolicy"] = map[string]any{"granular": map[string]any{}} },
		"no thread":       func(r map[string]any) { delete(r, "thread") },
	} {
		if _, err := ParseCodexThreadStart(threadStartResult(mutate), "/work"); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
