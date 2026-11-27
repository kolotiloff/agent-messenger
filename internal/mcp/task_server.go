package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

// The worker tools of one task run (T-024, T-026). The server is bound to one
// run and its receipt; the tools work only while that run is running and its
// conversation identity is confirmed. A report or question is stored
// provisionally and published only after the run is verified.

const taskInstructions = "Kotel worker tools for one task. Call read_task first. " +
	"End the run with exactly one of report, ask or ask_owner, then stop."

func taskTools() []map[string]any {
	images := map[string]any{"type": "array", "maxItems": core.MaxTaskImages, "items": map[string]any{"type": "string", "maxLength": 300},
		"description": "optional paths of pictures to show (PNG, JPEG, GIF or WebP, up to 5 MB each), such as a screenshot; relative paths are in the project folder"}
	text := func(desc string) map[string]any {
		return map[string]any{"type": "object", "additionalProperties": false, "required": []string{"text"},
			"properties": map[string]any{"text": map[string]any{"type": "string", "minLength": 1, "maxLength": core.MaxDispositionText, "description": desc}, "images": images}}
	}
	return []map[string]any{
		{"name": "read_task", "description": "Read your task: its text, project and folder. On a resumed run it returns your question and its answer instead of the task text, which this conversation already has; pass include_task: true to get the text again. owner_notes is what the owner wrote to you while you worked: take it into account.",
			"inputSchema": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
				"include_task": map[string]any{"type": "boolean", "description": "also return the task text on a resumed run"},
			}}},
		{"name": "report", "description": "Finish the task. For a direct question from the owner, put your answer to that question in text, in the owner's language. For a work assignment, describe what you did, verified and what remains, in at most 8 short lines. Put longer work details in a project file and list its path in files. Commit changes first; if you cannot commit, ask for help. Ends this run.",
			"inputSchema": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"text"},
				"properties": map[string]any{
					"text":    map[string]any{"type": "string", "minLength": 1, "maxLength": core.MaxDispositionText, "description": "the report, at most 8 short lines"},
					"changes": map[string]any{"type": "string", "maxLength": 500, "description": "the change you made: commit, changeset or branch; leave out when nothing changed"},
					"tests":   map[string]any{"type": "string", "maxLength": 500, "description": "the tests or checks you ran and their result, one line"},
					"files":   map[string]any{"type": "array", "maxItems": core.MaxReportFiles, "items": map[string]any{"type": "string", "maxLength": 300}, "description": "paths of files the coordinator should read, such as a detailed report"},
					"images":  images,
				}}},
		{"name": "ask", "description": "Ask the coordinator a question you cannot resolve yourself. Ends this run; the task resumes with the answer.",
			"inputSchema": text("the question")},
		{"name": "ask_owner", "description": "Ask the owner directly, when this project's rules allow it. Ends this run; the task resumes with the answer.",
			"inputSchema": text("the question")},
	}
}

// ServeTask runs the worker tool server for one run until in closes.
func ServeTask(dbPath, run, receipt string, in io.Reader, out io.Writer) error {
	store, err := core.OpenTaskRunStore(context.Background(), dbPath, run)
	if err != nil {
		return err
	}
	defer store.Close()
	return serveTask(store, run, receipt, in, out)
}

// serveTask answers MCP requests. Without a store it is the idle server of a
// Cursor session that Kotel did not start: it lists no tools.
func serveTask(store *core.Store, run, receipt string, in io.Reader, out io.Writer) error {
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
				"serverInfo":      map[string]any{"name": "kotel-task", "version": "0.2.0"},
				"instructions":    taskInstructions,
			}
		case "ping":
			result = map[string]any{}
		case "tools/list":
			if store == nil {
				result = map[string]any{"tools": []any{}}
			} else {
				result = map[string]any{"tools": taskTools()}
			}
		case "tools/call":
			if store == nil {
				result = map[string]any{"content": []map[string]any{{"type": "text", "text": "not a Kotel task run"}}, "isError": true}
				break
			}
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
	case core.DispositionReport:
		var parts core.ReportParts
		var text string
		var pics []core.TaskImageFile
		if pics, err = taskImageArgs(store, run, args); err != nil {
			break
		}
		delete(args, "images")
		if text, err = reportText(args, &parts); err != nil {
			break
		}
		if err = store.RecordDispositionImages(run, receipt, name, text, pics); err == nil {
			value = map[string]any{"recorded": name, "next": "Stop now. Kotel publishes this after it verifies the run."}
		}
	case core.DispositionAsk, core.DispositionAskOwner:
		var pics []core.TaskImageFile
		if pics, err = taskImageArgs(store, run, args); err != nil {
			break
		}
		delete(args, "images")
		text, ok := args["text"].(string)
		if !ok || len(args) != 1 {
			err = fmt.Errorf("%s takes text and optional images", name)
			break
		}
		if err = store.RecordDispositionImages(run, receipt, name, text, pics); err == nil {
			value = map[string]any{"recorded": name, "next": "Stop now. Kotel publishes this after it verifies the run."}
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

// reportText reads the report's arguments into one text (T-050).
func reportText(args map[string]any, parts *core.ReportParts) (string, error) {
	for key, v := range args {
		var ok bool
		switch key {
		case "text":
			parts.Text, ok = v.(string)
		case "changes":
			parts.Changes, ok = v.(string)
		case "tests":
			parts.Tests, ok = v.(string)
		case "files":
			list, isList := v.([]any)
			ok = isList
			for _, item := range list {
				path, isString := item.(string)
				if !isString {
					ok = false
					break
				}
				parts.Files = append(parts.Files, path)
			}
		}
		if !ok {
			return "", errors.New("report takes text and optional changes, tests (strings), files and images (lists of paths)")
		}
	}
	return parts.Compose()
}

// TaskEnvVariable carries a Cursor run's database, run and receipt to its
// worker tools (D-049). Cursor reads MCP servers only from its own mcp.json,
// so Kotel keeps one stable entry there that names this variable.
const TaskEnvVariable = adapters.TaskEnvVariable

// TaskFromEnv reads the run from the environment value. Anything else, an
// unset variable or a placeholder Cursor did not fill in, is not a run.
func TaskFromEnv(value string) (db, run, receipt string, ok bool) {
	var v struct {
		DB      string `json:"db"`
		Run     string `json:"run"`
		Receipt string `json:"receipt"`
	}
	if json.Unmarshal([]byte(value), &v) != nil || v.DB == "" || v.Run == "" || v.Receipt == "" {
		return "", "", "", false
	}
	return v.DB, v.Run, v.Receipt, true
}

// TaskMain is the entry for `messenger task-mcp --db DB --run RUN --receipt R`
// and for `messenger task-mcp --from-env` (Cursor, D-049).
func TaskMain(args []string) error {
	if len(args) == 1 && args[0] == "--from-env" {
		db, run, receipt, ok := TaskFromEnv(os.Getenv(TaskEnvVariable))
		if !ok {
			return serveTask(nil, "", "", os.Stdin, os.Stdout)
		}
		return ServeTask(db, run, receipt, os.Stdin, os.Stdout)
	}
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

// taskImageArgs reads the pictures a worker names (T-175). Relative paths
// are in the task's folder. Only regular files are read, and the store
// keeps only what is a picture by its content.
func taskImageArgs(store *core.Store, run string, args map[string]any) ([]core.TaskImageFile, error) {
	raw, present := args["images"]
	if !present {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok || len(list) > core.MaxTaskImages {
		return nil, fmt.Errorf("images is a list of at most %d paths", core.MaxTaskImages)
	}
	folder, err := store.RunFolder(run)
	if err != nil {
		return nil, err
	}
	var out []core.TaskImageFile
	for _, item := range list {
		path, ok := item.(string)
		if !ok || strings.TrimSpace(path) == "" {
			return nil, errors.New("images is a list of paths")
		}
		if !filepath.IsAbs(path) {
			if folder == "" {
				return nil, fmt.Errorf("%s: give the full path", path)
			}
			path = filepath.Join(folder, path)
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("picture %s: %v", path, err)
		}
		if !info.Mode().IsRegular() || info.Size() > core.MaxTaskImage {
			return nil, fmt.Errorf("picture %s must be a file of at most %d MB", path, core.MaxTaskImage>>20)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("picture %s: %v", path, err)
		}
		out = append(out, core.TaskImageFile{Name: filepath.Base(path), Data: data})
	}
	return out, nil
}
