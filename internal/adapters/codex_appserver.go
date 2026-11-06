package adapters

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
)

var codexDisabledFeatures = []string{
	"shell_tool", "shell_snapshot", "apps", "plugins", "hooks", "multi_agent",
	"browser_use", "computer_use", "goals", "remote_plugin", "remote_control", "memories",
}

// CodexAppServerArgs is for an isolated, managed profile only. The caller must
// supply a private HOME/CODEX_HOME/workspace and a sanitized environment, retain
// ownership of the process, and check config/read before creating any thread.
// These flags alone are NOT an authorization to call a model.
func CodexAppServerArgs(cli string) ([]string, error) {
	if !filepath.IsAbs(cli) {
		return nil, errors.New("Codex CLI path must be absolute")
	}
	args := []string{cli, "app-server", "--listen", "stdio://"}
	for _, feature := range codexDisabledFeatures {
		args = append(args, "--disable", feature)
	}
	for _, override := range []string{
		`cli_auth_credentials_store="file"`,
		`sandbox_mode="read-only"`, `approval_policy="never"`, `web_search="disabled"`,
		`mcp_servers={}`,
	} {
		args = append(args, "-c", override)
	}
	return args, nil
}

// ValidateCodexAppServerConfig checks the minimum model-free preflight contract
// against config/read.result.config. It deliberately rejects incomplete reports.
// A pass does not prove all tool isolation: thread creation, configuration changes,
// saved tools, process ownership and authorization require separate checks.
// Error strings never include configuration values, server names or credentials.
func ValidateCodexAppServerConfig(raw json.RawMessage) error {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(raw, &config); err != nil || config == nil {
		return errors.New("invalid Codex configuration report")
	}
	for key, expected := range map[string]string{
		"sandbox_mode": "read-only", "approval_policy": "never", "web_search": "disabled",
	} {
		var value string
		if json.Unmarshal(config[key], &value) != nil || value != expected {
			return fmt.Errorf("Codex preflight requires %s", key)
		}
	}
	var features map[string]json.RawMessage
	if json.Unmarshal(config["features"], &features) != nil || features == nil {
		return errors.New("Codex feature configuration is missing")
	}
	for _, feature := range codexDisabledFeatures {
		// json.Unmarshal(null, *bool) succeeds; compare the literal instead.
		var value any
		if json.Unmarshal(features[feature], &value) != nil || value != false {
			return fmt.Errorf("Codex preflight requires disabled feature %s", feature)
		}
	}
	var servers map[string]json.RawMessage
	if json.Unmarshal(config["mcp_servers"], &servers) != nil || servers == nil {
		return errors.New("Codex MCP configuration is missing")
	}
	for _, rawServer := range servers {
		var server map[string]json.RawMessage
		if json.Unmarshal(rawServer, &server) != nil || server == nil {
			return errors.New("invalid Codex MCP configuration")
		}
		var enabled any
		if json.Unmarshal(server["enabled"], &enabled) != nil || enabled != false {
			return errors.New("Codex preflight found an enabled or unconfirmed MCP server")
		}
	}
	return nil
}

// CodexConfigFingerprint hashes the effective config in a canonical form.
// app-server does not keep key order stable between config/read calls (seen
// with Codex 0.155.1), so the raw bytes cannot be compared. Objects are
// re-encoded with sorted keys and numbers kept as written.
func CodexConfigFingerprint(raw json.RawMessage) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", errors.New("invalid Codex configuration report")
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}
