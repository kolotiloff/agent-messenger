package mcp

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"net/http"
)

// StreamHandler is bound to one process owner. SQLite authorizes the current
// turn and receipt on every call, including after stop or supervisor restart.
type StreamHandler struct {
	Store  *core.Store
	Run    string
	Secret string
}

func (g *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/mcp" || r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+g.Secret)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params map[string]any  `json:"params"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "bad JSON", 400)
		return
	}
	if len(req.ID) == 0 || string(req.ID) == "null" {
		w.WriteHeader(202)
		return
	}
	var result any
	switch req.Method {
	case "initialize":
		version, _ := req.Params["protocolVersion"].(string)
		if version != "2025-03-26" && version != "2025-06-18" {
			version = "2025-06-18"
		}
		result = map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "agent-messenger", "version": "0.2"}}
	case "ping":
		result = map[string]any{}
	case "tools/list":
		result = map[string]any{"tools": []any{
			map[string]any{"name": "read_current", "description": "Read the assigned message and its one-turn receipt.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}},
			map[string]any{"name": "reply", "description": "Reply using the receipt returned by read_current. Routing is assigned by the host.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string", "minLength": 1, "maxLength": 2000}, "receipt": map[string]any{"type": "string"}}, "required": []string{"text", "receipt"}, "additionalProperties": false}},
		}}
	case "tools/call":
		name, _ := req.Params["name"].(string)
		args, _ := req.Params["arguments"].(map[string]any)
		value, err := g.tool(name, args)
		data, _ := json.Marshal(value)
		if err != nil {
			data = []byte(err.Error())
		}
		result = map[string]any{"content": []any{map[string]any{"type": "text", "text": string(data)}}, "isError": err != nil}
	default:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "Method not found"}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}

func (g *StreamHandler) tool(name string, args map[string]any) (any, error) {
	switch name {
	case "read_current":
		if len(args) != 0 {
			return nil, errors.New("read_current takes no arguments")
		}
		return g.Store.StreamRead(g.Run)
	case "reply":
		receipt, ok := args["receipt"].(string)
		text, valid := args["text"].(string)
		if len(args) != 2 || !ok || !valid {
			return nil, errors.New("reply requires only text and receipt; route is assigned by the host")
		}
		return g.Store.StreamReply(g.Run, receipt, text)
	default:
		return nil, errors.New("unknown tool")
	}
}
