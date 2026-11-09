package mcp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Outside a Messenger run (an ordinary Cursor session), the worker tool
// server from mcp.json lists nothing and calls nothing.
func TestTaskServerFromEnvOutsideARun(t *testing.T) {
	for _, value := range []string{"", "${env:AGENT_MESSENGER_TASK}", `{"db":"x"}`} {
		if _, _, _, ok := TaskFromEnv(value); ok {
			t.Fatalf("%q taken as a run", value)
		}
	}
	if db, run, receipt, ok := TaskFromEnv(`{"db":"/d","run":"r","receipt":"x"}`); !ok || db != "/d" || run != "r" || receipt != "x" {
		t.Fatal("a run was not read")
	}
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"report","arguments":{"text":"x"}}}` + "\n")
	var out bytes.Buffer
	if err := serveTask(nil, "", "", in, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"tools":[]`) || !strings.Contains(lines[1], `"isError":true`) {
		t.Fatalf("%s", out.String())
	}
}

// A worker run that loads the owner's coordinator connection from the project
// folder gets no coordinator tools and does not open the database (D-050).
func TestCoordinatorInsideAWorkerRunHasNoTools(t *testing.T) {
	db := filepath.Join(t.TempDir(), "never-created.sqlite")
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"claude-code"}}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"start_task","arguments":{}}}` + "\n")
	var out bytes.Buffer
	if err := runCoordinator(db, "Game", "run-1", in, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || !strings.Contains(lines[1], `"tools":[]`) || !strings.Contains(lines[2], `"isError":true`) || strings.Contains(out.String(), "start_task") {
		t.Fatalf("%s", out.String())
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatal("the database was opened inside a worker run")
	}
}
