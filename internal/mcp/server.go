// Package mcp is the reply-only stdio server for one execution token.
package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/kolotiloff/agent-messenger/internal/core"
)

type rpcRequest struct {
	ID     any            `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

// Serve reads newline-delimited JSON-RPC from in and writes responses to out.
// The model can only read the assigned message and reply with text.
func Serve(dbPath, token string, in io.Reader, out io.Writer) error {
	store, err := core.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	return serveStore(store, token, in, out)
}

func serveStore(store *core.Store, token string, in io.Reader, out io.Writer) error {
	tools := []map[string]any{
		{"name": "read_current", "description": "Read the message assigned to this execution.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}},
		{"name": "reply", "description": "Reply to the assigned message. Routing is automatic.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{
				"text": map[string]any{"type": "string", "minLength": 1, "maxLength": 2000},
			}, "required": []string{"text"}, "additionalProperties": false}},
	}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var request rpcRequest
		if err := json.Unmarshal(line, &request); err != nil {
			continue
		}
		if request.ID == nil {
			continue
		}
		var result any
		var rpcErr any
		switch request.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "agent-messenger", "version": "0.1.0"},
			}
		case "ping":
			result = map[string]any{}
		case "tools/list":
			result = map[string]any{"tools": tools}
		case "tools/call":
			result = callTool(store, token, request.Params)
		default:
			rpcErr = map[string]any{"code": -32601, "message": "Method not found"}
		}
		response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		if rpcErr != nil {
			response["error"] = rpcErr
		} else {
			response["result"] = result
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s\n", encoded); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func callTool(store *core.Store, token string, params map[string]any) map[string]any {
	name, _ := params["name"].(string)
	args, _ := params["arguments"].(map[string]any)
	var value any
	var err error
	switch name {
	case "read_current":
		if len(args) != 0 {
			err = errors.New("read_current takes no arguments")
			break
		}
		value, err = store.CurrentMessage(token)
	case "reply":
		if len(args) != 1 {
			err = errors.New("only reply(text) is allowed")
			break
		}
		text, ok := args["text"].(string)
		if !ok {
			err = errors.New("only reply(text) is allowed")
			break
		}
		value, err = store.Reply(token, text)
	default:
		err = errors.New("only read_current() and reply(text) are allowed")
	}
	if err != nil {
		return map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}
	}
	encoded, _ := json.Marshal(value)
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(encoded)}}, "isError": false}
}

// Main is the product entry for `messenger mcp`.
func Main(args []string) error {
	var db, token string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--db":
			i++
			if i < len(args) {
				db = args[i]
			}
		case "--token":
			i++
			if i < len(args) {
				token = args[i]
			}
		}
	}
	if db == "" || token == "" {
		return errors.New("mcp requires --db and --token")
	}
	return Serve(db, token, os.Stdin, os.Stdout)
}
