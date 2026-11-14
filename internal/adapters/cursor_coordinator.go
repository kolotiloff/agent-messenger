package adapters

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// CursorProjectConfig is shared by the editor and CLI for this folder (T-162).
func CursorProjectConfig(folder string) string { return filepath.Join(folder, ".cursor", "mcp.json") }

// CursorCoordinatorLink only inspects the project entry. Cursor's separate
// approval/disabled-server lists remain under the owner's control.
func CursorCoordinatorLink(path, messenger, root, project string) (string, error) {
	raw, _, err := readCursorCoordinatorConfig(path)
	if err != nil {
		return LinkUnknown, err
	}
	state, _, _, _, err := classifyCursorCoordinator(raw, messenger, root, project)
	return state, err
}

// EnsureCursorCoordinator changes only our server entry. Other JSON values
// survive; foreign, disabled, ambiguous and unreadable files stay byte-for-byte.
func EnsureCursorCoordinator(path, messenger, root, project string) error {
	if !filepath.IsAbs(path) || !filepath.IsAbs(messenger) {
		return errors.New("cursor config and messenger need absolute paths")
	}
	raw, mode, err := readCursorCoordinatorConfig(path)
	if err != nil {
		return err
	}
	state, top, servers, entry, err := classifyCursorCoordinator(raw, messenger, root, project)
	if err != nil {
		return err
	}
	if state != LinkMissing && state != LinkStale {
		return nil
	}
	if entry == nil {
		entry = map[string]json.RawMessage{}
		entry["type"], _ = json.Marshal("stdio")
		entry["args"], _ = json.Marshal(CoordinatorArgs(root, project))
		// As with messenger_task, explicitly forward the worker guard through
		// Cursor's interpolation. Ordinary coordinator chats leave it empty.
		entry["env"], _ = json.Marshal(map[string]string{WorkerRunVariable: "${env:" + WorkerRunVariable + "}"})
	}
	entry["command"], _ = json.Marshal(messenger)
	servers[CoordinatorServerName], _ = json.Marshal(entry)
	top["mcpServers"], _ = json.Marshal(servers)
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".messenger-mcp-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(mode); err == nil {
		_, err = tmp.Write(append(out, '\n'))
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	// Do not overwrite an edit made while the new file was prepared.
	current, _, err := readCursorCoordinatorConfig(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, raw) {
		return errors.New("cursor config changed while connecting; nothing was replaced")
	}
	return platform.ReplaceFile(tmp.Name(), path)
}

func readCursorCoordinatorConfig(path string) ([]byte, os.FileMode, error) {
	mode := os.FileMode(0o600)
	for _, name := range []string{filepath.Dir(path), path} {
		info, err := os.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, mode, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, mode, errors.New("cursor config path is a symlink")
		}
		if name == path {
			if !info.Mode().IsRegular() || info.Size() > 1<<20 {
				return nil, mode, errors.New("cursor config is not a regular file of at most 1 MB")
			}
			mode = info.Mode().Perm()
		} else if !info.IsDir() {
			return nil, mode, errors.New("cursor config parent is not a directory")
		}
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, mode, nil
	}
	if err != nil {
		return nil, mode, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err == nil && len(raw) > 1<<20 {
		err = errors.New("cursor config exceeds 1 MB")
	}
	return raw, mode, err
}

// Decode only the objects we edit, refusing duplicate keys instead of silently
// choosing one. Unknown values remain RawMessage, including large JSON numbers.
func cursorConfigObject(raw []byte) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("cursor config needs a JSON object")
	}
	result := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, errors.New("cursor config has a non-string key")
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("cursor config has duplicate key %q", key)
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return nil, err
		}
		result[key] = value
	}
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errors.New("cursor config has trailing data")
	}
	return result, nil
}

func classifyCursorCoordinator(raw []byte, messenger, root, project string) (state string, top, servers, entry map[string]json.RawMessage, err error) {
	state = LinkUnknown
	top, servers = map[string]json.RawMessage{}, map[string]json.RawMessage{}
	if raw != nil {
		top, err = cursorConfigObject(raw)
		if err != nil {
			return
		}
	}
	if value, ok := top["mcpServers"]; ok {
		servers, err = cursorConfigObject(value)
		if err != nil {
			return
		}
	}
	value, ok := servers[CoordinatorServerName]
	if !ok {
		state = LinkMissing
		return
	}
	entry, err = cursorConfigObject(value)
	if err != nil {
		return
	}
	var server struct {
		Type     string   `json:"type"`
		Command  string   `json:"command"`
		Args     []string `json:"args"`
		URL      string   `json:"url"`
		Disabled bool     `json:"disabled"`
		Enabled  *bool    `json:"enabled"`
	}
	if err = json.Unmarshal(value, &server); err != nil {
		return
	}
	if server.URL != "" || (server.Type != "" && server.Type != "stdio") || server.Command == "" || !slices.Equal(server.Args, CoordinatorArgs(root, project)) {
		state = LinkOther
		return
	}
	if server.Disabled || (server.Enabled != nil && !*server.Enabled) {
		state = LinkOff
		return
	}
	if sameTrustPath(server.Command, messenger) {
		state = LinkConnected
	} else {
		state = LinkStale
	}
	return
}
