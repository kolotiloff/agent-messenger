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

// The worker tools of one task run (T-024, T-026). The server is bound to one
// run and its receipt; the tools work only while that run is running and its
// conversation identity is confirmed. A report or question is stored
// provisionally and published only after the run is verified.

const taskInstructions = "Agent Messenger worker tools for one task. Call read_task first. " +
	"End the run with exactly one of report, ask or ask_owner, then stop."

func taskTools() []map[string]any {
	text := func(desc string) map[string]any {
		return map[string]any{"type": "object", "additionalProperties": false, "required": []string{"text"},
			"properties": map[string]any{"text": map[string]any{"type": "string", "minLength": 1, "maxLength": core.MaxDispositionText, "description": desc}}}
	}
	return []map[string]any{
		{"name": "read_task", "description": "Read your task: its text, project and folder. On a resumed run it returns your question and its answer instead of the task text, which this conversation already has; pass include_task: true to get the text again.",
			"inputSchema": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
				"include_task": map[string]any{"type": "boolean", "description": "also return the task text on a resumed run"},
			}}},
		{"name": "report", "description": "Finish the task: what you did, what you verified and what remains. Ends this run; the task is done.",
			"inputSchema": text("the report")},
		{"name": "ask", "description": "Ask the coordinator a question you cannot resolve yourself. Ends this run; the task resumes with the answer.",
			"inputSchema": text("the question")},
		{"name": "ask_owner", "description": "Ask the owner directly, when this project's rules allow it. Ends this run; the task resumes with the answer.",
			"inputSchema": text("the question")},
	}
}

// ServeTask runs the worker tool server for one run until in closes.
func ServeTask(dbPath, run, receipt string, in io.Reader, out io.Writer) error {
	store, err := core.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var request rpcRequest
		if json.Unmarshal(scanner.Bytes(), &request) != nil || request.ID == nil {
			continue
		}
		var result any
		var rpcErr any
		switch request.Method {
		case "initialize":
			version, _ := request.Params["protocolVersion"].(string)
			if version == "" {
				version = "2025-06-18"
			}
			result = map[string]any{
				"protocolVersion": version,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "agent-messenger-task", "version": "0.2.0"},
				"instructions":    taskInstructions,
			}
		case "ping":
			result = map[string]any{}
		case "tools/list":
			result = map[string]any{"tools": taskTools()}
		case "tools/call":
			result = callTaskTool(store, run, receipt, request.Params)
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

func callTaskTool(store *core.Store, run, receipt string, params map[string]any) map[string]any {
	name, _ := params["name"].(string)
	args, _ := params["arguments"].(map[string]any)
	var value any
	var err error
	switch name {
	case "read_task":
		include, isBool := args["include_task"].(bool)
		if len(args) > 1 || (len(args) == 1 && !isBool) {
			err = errors.New("read_task takes only include_task")
			break
		}
		value, err = store.ReadTask(run, receipt, include)
	case core.DispositionReport, core.DispositionAsk, core.DispositionAskOwner:
		text, ok := args["text"].(string)
		if !ok || len(args) != 1 {
			err = fmt.Errorf("%s takes only text", name)
			break
		}
		if err = store.RecordDisposition(run, receipt, name, text); err == nil {
			value = map[string]any{"recorded": name, "next": "Stop now. Messenger publishes this after it verifies the run."}
		}
	default:
		err = errors.New("unknown tool")
	}
	if err != nil {
		return map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}
	}
	encoded, _ := json.Marshal(value)
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(encoded)}}, "isError": false}
}

// TaskMain is the entry for `messenger task-mcp --db DB --run RUN --receipt R`.
func TaskMain(args []string) error {
	values := map[string]string{}
	for i := 0; i+1 < len(args); i += 2 {
		values[args[i]] = args[i+1]
	}
	db, run, receipt := values["--db"], values["--run"], values["--receipt"]
	if db == "" || run == "" || receipt == "" {
		return errors.New("task-mcp requires --db, --run and --receipt")
	}
	return ServeTask(db, run, receipt, os.Stdin, os.Stdout)
}
