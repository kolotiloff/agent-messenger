package adapters

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in installed-CLI test. Never uses the user's profile or starts a model.
func TestInstalledCodexAppServerPreflight(t *testing.T) {
	cli := os.Getenv("MESSENGER_CODEX_PROBE")
	if cli == "" {
		t.Skip("set MESSENGER_CODEX_PROBE to an absolute Codex executable for model-free checks")
	}
	for _, tc := range []struct {
		name                       string
		fixture, disable, wantPass bool
	}{
		{"clean", false, false, true},
		{"inherited-server", true, false, false},
		{"explicitly-disabled-server", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "home")
			profile := filepath.Join(home, ".codex")
			workspace := filepath.Join(root, "workspace")
			for _, dir := range []string{profile, workspace} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if tc.fixture {
				if err := os.WriteFile(filepath.Join(profile, "config.toml"), []byte("[mcp_servers.fixture]\ncommand = \"messenger-fixture-never-run\"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args, err := CodexAppServerArgs(cli)
			if err != nil {
				t.Fatal(err)
			}
			if tc.disable {
				args = append(args, "-c", "mcp_servers.fixture.enabled=false")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, args[0], args[1:]...)
			cmd.Dir = workspace
			cmd.Env = []string{"HOME=" + home, "CODEX_HOME=" + profile, "TMPDIR=" + root, "PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
			cmd.Stderr = io.Discard
			input, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			output, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { cancel(); _ = cmd.Wait() }()
			scanner := bufio.NewScanner(output)
			scanner.Buffer(make([]byte, 4096), 4<<20)
			if _, err = probeRPC(input, scanner, 1, "initialize", map[string]any{
				"clientInfo":   map[string]any{"name": "messenger_go_preflight", "version": "0.1"},
				"capabilities": map[string]any{"experimentalApi": true},
			}); err != nil {
				t.Fatal(err)
			}
			if _, err = io.WriteString(input, "{\"method\":\"initialized\"}\n"); err != nil {
				t.Fatal(err)
			}
			result, err := probeRPC(input, scanner, 2, "config/read", map[string]any{"includeLayers": false, "cwd": workspace})
			if err != nil {
				t.Fatal(err)
			}
			var report struct {
				Config json.RawMessage `json:"config"`
			}
			if err = json.Unmarshal(result, &report); err != nil {
				t.Fatal("invalid config response")
			}
			err = ValidateCodexAppServerConfig(report.Config)
			if (err == nil) != tc.wantPass {
				t.Fatalf("preflight pass=%v, wanted %v: %v", err == nil, tc.wantPass, err)
			}
			if err = input.Close(); err != nil {
				t.Fatal(err)
			}
			if err = cmd.Wait(); err != nil {
				t.Fatal("app-server did not exit cleanly")
			}
		})
	}
}

// Only these two model-free request methods are possible in this test harness.
func probeRPC(w io.Writer, scanner *bufio.Scanner, id int, method string, params any) (json.RawMessage, error) {
	if method != "initialize" && method != "config/read" {
		return nil, fmt.Errorf("probe method not allowed")
	}
	data, err := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	n, err := w.Write(data)
	if err != nil {
		return nil, err
	}
	if n != len(data) {
		return nil, io.ErrShortWrite
	}
	for count := 0; count < 64 && scanner.Scan(); count++ {
		var frame struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &frame) != nil {
			return nil, fmt.Errorf("malformed probe response")
		}
		if len(frame.ID) == 0 && frame.Method != "" && len(frame.Result) == 0 && len(frame.Error) == 0 {
			continue
		}
		var responseID int
		if len(frame.ID) == 0 || string(frame.ID) == "null" || json.Unmarshal(frame.ID, &responseID) != nil || responseID != id || frame.Method != "" || len(frame.Error) != 0 || len(frame.Result) == 0 {
			return nil, fmt.Errorf("unexpected probe response; no retry")
		}
		return frame.Result, nil
	}
	return nil, fmt.Errorf("missing probe response; no retry")
}

func TestCodexProbeRejectsAmbiguousProtocolAndModelMethods(t *testing.T) {
	for _, wire := range []string{
		`{"id":9,"result":{}}`,
		`{"id":2,"method":"item/tool/call","params":{}}`,
		`{"id":null,"method":"item/tool/call","params":{}}`,
		`{"id":2,"result":{},"error":{"code":-1}}`,
		`{"id":2,"error":{"code":-1,"message":"private-data"}}`,
		`{"id":2`,
		`{}`,
		"",
	} {
		var output bytes.Buffer
		if _, err := probeRPC(&output, bufio.NewScanner(strings.NewReader(wire)), 2, "config/read", map[string]any{}); err == nil {
			t.Fatal("ambiguous protocol accepted")
		} else if strings.Contains(err.Error(), "private-data") {
			t.Fatal("server data leaked")
		}
		if strings.Count(output.String(), "\n") != 1 {
			t.Fatal("probe retried a request")
		}
	}
	var output bytes.Buffer
	if _, err := probeRPC(&output, bufio.NewScanner(strings.NewReader("")), 2, "turn/start", map[string]any{}); err == nil || output.Len() != 0 {
		t.Fatal("model method escaped probe allowlist")
	}
}
