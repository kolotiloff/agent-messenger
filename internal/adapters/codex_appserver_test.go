package adapters

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func validCodexConfig() map[string]any {
	features := map[string]any{}
	for _, feature := range codexDisabledFeatures {
		features[feature] = false
	}
	return map[string]any{"features": features, "mcp_servers": map[string]any{},
		"sandbox_mode": "read-only", "approval_policy": "never", "web_search": "disabled"}
}

func TestCodexPreflightRejectsIncompleteOrInheritedCapabilities(t *testing.T) {
	tests := []struct {
		name string
		edit func(map[string]any)
	}{
		{"enabled server", func(c map[string]any) {
			c["mcp_servers"] = map[string]any{"private-server": map[string]any{"enabled": true, "token": "DO-NOT-LOG"}}
		}},
		{"implicit server", func(c map[string]any) {
			c["mcp_servers"] = map[string]any{"private-server": map[string]any{"command": "DO-NOT-LOG"}}
		}},
		{"null enabled", func(c map[string]any) {
			c["mcp_servers"] = map[string]any{"private-server": map[string]any{"enabled": nil}}
		}},
		{"null server", func(c map[string]any) { c["mcp_servers"] = map[string]any{"private-server": nil} }},
		{"missing servers", func(c map[string]any) { delete(c, "mcp_servers") }},
		{"null servers", func(c map[string]any) { c["mcp_servers"] = nil }},
		{"missing features", func(c map[string]any) { delete(c, "features") }},
		{"missing hooks", func(c map[string]any) { delete(c["features"].(map[string]any), "hooks") }},
		{"null hooks", func(c map[string]any) { c["features"].(map[string]any)["hooks"] = nil }},
		{"enabled goals", func(c map[string]any) { c["features"].(map[string]any)["goals"] = true }},
		{"string false", func(c map[string]any) { c["features"].(map[string]any)["shell_tool"] = "false" }},
		{"write sandbox", func(c map[string]any) { c["sandbox_mode"] = "workspace-write" }},
		{"live web", func(c map[string]any) { c["web_search"] = "live" }},
		{"missing policy", func(c map[string]any) { delete(c, "approval_policy") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validCodexConfig()
			tt.edit(config)
			data, _ := json.Marshal(config)
			err := ValidateCodexAppServerConfig(data)
			if err == nil {
				t.Fatal("unsafe or incomplete configuration accepted")
			}
			if strings.Contains(err.Error(), "DO-NOT-LOG") || strings.Contains(err.Error(), "private-server") {
				t.Fatal("configuration leaked into error")
			}
		})
	}
	for _, raw := range []string{"null", "[]", "true", "{", "{}"} {
		if ValidateCodexAppServerConfig(json.RawMessage(raw)) == nil {
			t.Fatalf("invalid report accepted: %s", raw)
		}
	}
}

func TestCodexPreflightAcceptsExplicitlyDisabledServers(t *testing.T) {
	config := validCodexConfig()
	for _, servers := range []any{map[string]any{}, map[string]any{"fixture": map[string]any{"enabled": false}}} {
		config["mcp_servers"] = servers
		data, _ := json.Marshal(config)
		if err := ValidateCodexAppServerConfig(data); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCodexAppServerArgsUseIsolatedStdio(t *testing.T) {
	if _, err := CodexAppServerArgs("codex"); err == nil {
		t.Fatal("relative CLI accepted")
	}
	cli := filepath.Join(t.TempDir(), "codex")
	args, err := CodexAppServerArgs(cli)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, "\n")
	if !strings.Contains(joined, "app-server\n--listen\nstdio://") {
		t.Fatal("must own stdio server")
	}
	for _, forbidden := range []string{"--ignore-user-config", "--resume", "--continue", "turn/start"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("unexpected %s", forbidden)
		}
	}
}
