package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

// The coordinator's MCP server (T-025). One server serves one project: the
// coordinator chat connects with the project name, and wait, tasks and
// list_agents answer for that project only (D-026). Each tools/call runs in
// its own goroutine, so a blocked wait does not stop other calls, ping or
// cancellation on the same connection. The contract follows the T-023 wait
// pilot: a replayable cursor that the server never advances on its own,
// progress notifications every 60 s, and cancellation without losing events.

const (
	coordinatorDefaultWait = 600
	coordinatorMaxWait     = 3600
	coordinatorMaxReturned = 20
	// coordinatorWaitBudget bounds the text one wait returns (T-031), in
	// characters. One event is always returned, whatever its size.
	coordinatorWaitBudget = 24000
	// coordinatorTaskLimit is the default and coordinatorTaskMax the largest
	// number of tasks one tasks call lists (T-032).
	coordinatorTaskLimit = 50
	coordinatorTaskMax   = 200
)

// coordinatorPoll and coordinatorProgress are variables so tests can shorten them.
var (
	coordinatorPoll     = 300 * time.Millisecond
	coordinatorProgress = 60 * time.Second
	// coordinatorLogWait is how long get_log waits for the other computer.
	coordinatorLogWait = 20 * time.Second
)

const coordinatorInstructions = `Agent Messenger: start tasks for worker agents in this project and wait for their reports.
- list_agents shows this project's computers, their project folder and agents: this computer and the other computers of its network. A task for another computer is sent to it and runs there; it fails with the reason if that computer does not take tasks from other computers or has no folder for the project. Another computer shows accepts_tasks and has_project when it reported them; do not send a task where either is false, and tell the owner instead. Questions, reports and outcomes come back through wait as usual.
- start_task starts one task in a new worker session. Give every task a unique op_id and reuse it only to retry the same call after a lost response.
- wait blocks until something happens in this project's tasks: a report, a question for you, a failure, an interruption, a block or a cancellation. Always pass after = the cursor from the previous wait. When it returns events, act on them (answer questions with answer, tell the owner about reports), then call wait again with the new cursor. When it times out with no events, call wait again without comment. If it says more: true, more events are ready: call wait again at once with the new cursor.
- start_task takes an optional model. Leave it out for the agent's default. For Claude use opus for hard work (design, tricky bugs, large refactors), sonnet for ordinary coding, haiku for small mechanical work. For Codex and Cursor pass only a name from that agent's models in list_agents (the owner lists them in the window); if the list is empty, leave model out. Never guess a name: an unknown model fails the run. Tell the owner when you chose a non-default model and why.
- answer answers a worker's question (pass question_id and a unique op_id); the worker continues in the same session.
- Never start a task again by yourself after it failed, was interrupted or blocked; tell the owner and let them decide.
- list_agents shows each agent's limit when it reported one: state ok, warning (close to the limit) or exhausted, used_percent when the agent gave it, resets_at and observed_at. It is what the agent said at the end of its last run, not a live reading. Prefer an agent whose limit is not exhausted, and tell the owner when one is close to or at its limit.
- To follow up on a finished task in the same worker conversation (it keeps its context), use continue_task instead of a new start_task. It works for tasks of this computer; for another computer's task start a new one.
- Messenger already tells every worker to keep its report to a few lines, to put long details in a file and to follow the owner's rules for this project. A report ends with labelled lines when the worker gave them: «Изменения:» (commit or changeset), «Тесты:» and «Файлы:» (paths to read when needed). Do not ask workers for a «taken» line or a limit line: the task's state shows when it was accepted, and limits come from list_agents.
- get_log reads the end of another computer's Messenger log, for debugging Messenger itself (a task that never arrived, a refusal you cannot explain). That computer answers only if its owner turned on «Отдавать журнал по сети»; otherwise the reply says so. Session ids, codes and keys are removed. If no answer came in time, call get_log again with request_id. Do not use it to watch workers: wait does that.
- To tell a working task from a stuck one, call tasks: an open task shows the worker's last action and when it was (last_activity, last_activity_at; for another computer's task, the last progress line it forwarded). A long quiet time is a reason to tell the owner, not to cancel or start the task again by yourself. tokens is what the task used so far.
- When two tasks must not use one resource of a computer at the same time (the Unity editor, a device, a long build), give both start_task the same lock, such as unity-editor. The later task then waits to start until the earlier one is done, failed or cancelled; tasks shows waiting_for_lock with the task it waits for. A blocked task keeps its lock until the owner resolves it.
- A failure caused by a usage or rate limit says so and gives the reset time when the agent reported one. Tell the owner that time; do not start the task again before it.
How work is planned and split is decided with the owner under this project's own rules.`

type coordinator struct {
	store   *core.Store
	project string
	out     sync.Mutex
	w       io.Writer
	mu      sync.Mutex
	pending map[string]context.CancelFunc
}

// ServeCoordinator runs the coordinator MCP server for one project until in closes.
func ServeCoordinator(store *core.Store, project string, in io.Reader, out io.Writer) error {
	c := &coordinator{store: store, project: project, w: out, pending: map[string]context.CancelFunc{}}
	var wg sync.WaitGroup
	defer wg.Wait()
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &req) != nil {
			continue
		}
		hasID := len(req.ID) > 0 && string(req.ID) != "null"
		switch {
		case req.Method == "notifications/cancelled":
			var p struct {
				RequestID json.RawMessage `json:"requestId"`
			}
			_ = json.Unmarshal(req.Params, &p)
			c.mu.Lock()
			if cancel := c.pending[string(p.RequestID)]; cancel != nil {
				cancel()
			}
			c.mu.Unlock()
		case !hasID:
		case req.Method == "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
				ClientInfo      struct {
					Name string `json:"name"`
				} `json:"clientInfo"`
			}
			_ = json.Unmarshal(req.Params, &p)
			if p.ProtocolVersion == "" {
				p.ProtocolVersion = "2025-06-18"
			}
			_ = c.store.TouchCoordinatorClient(c.project, p.ClientInfo.Name)
			c.respond(req.ID, map[string]any{
				"protocolVersion": p.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "agent-messenger", "version": "0.2.0"},
				"instructions":    coordinatorInstructions + "\nProject: " + c.project,
			}, nil)
		case req.Method == "ping":
			c.respond(req.ID, map[string]any{}, nil)
		case req.Method == "tools/list":
			c.respond(req.ID, map[string]any{"tools": coordinatorTools()}, nil)
		case req.Method == "tools/call":
			ctx, cancel := context.WithCancel(context.Background())
			key := string(req.ID)
			c.mu.Lock()
			c.pending[key] = cancel
			c.mu.Unlock()
			wg.Add(1)
			go func(id, params json.RawMessage) {
				defer wg.Done()
				defer func() {
					c.mu.Lock()
					delete(c.pending, key)
					c.mu.Unlock()
					cancel()
				}()
				result, err := c.call(ctx, params)
				if ctx.Err() != nil {
					return // cancelled: MCP says not to answer
				}
				if err != nil {
					c.respond(id, coordinatorResult(err.Error(), true), nil)
					return
				}
				raw, _ := json.Marshal(result)
				c.respond(id, coordinatorResult(string(raw), false), nil)
			}(req.ID, req.Params)
		default:
			c.respond(req.ID, nil, map[string]any{"code": -32601, "message": "Method not found"})
		}
	}
	c.mu.Lock()
	for _, cancel := range c.pending {
		cancel()
	}
	c.mu.Unlock()
	return scanner.Err()
}

func coordinatorResult(text string, isError bool) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isError}
}

func coordinatorTools() []map[string]any {
	obj := func(props map[string]any, required ...string) map[string]any {
		schema := map[string]any{"type": "object", "additionalProperties": false, "properties": props}
		if len(required) > 0 {
			schema["required"] = required
		}
		return schema
	}
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	return []map[string]any{
		{"name": "list_agents", "description": "List this project's computers (this one and the other computers of its network), the project folder on this one, and their agents with how many tasks they run, the model names the owner listed for them and their last reported usage limit. Another computer also shows accepts_tasks and has_project (this project's folder) when it reported them.",
			"inputSchema": obj(map[string]any{})},
		{"name": "start_task", "description": "Start a task in a new worker session of the chosen agent, in this project's folder on the computer. Returns the task id. Repeating a call with the same op_id returns the same task.",
			"inputSchema": obj(map[string]any{
				"op_id":    str("unique id of this operation, for safe retries"),
				"agent":    map[string]any{"type": "string", "enum": []string{"claude", "codex", "cursor"}},
				"text":     map[string]any{"type": "string", "minLength": 1, "maxLength": core.MaxTaskText, "description": "the task: what to do and what to report"},
				"title":    map[string]any{"type": "string", "maxLength": core.MaxTaskTitle, "description": "short title shown in the window"},
				"computer": str("computer id from list_agents: this computer (the default) or another computer of the network"),
				"model":    map[string]any{"type": "string", "maxLength": 80, "description": "optional model for the worker, such as opus, sonnet or haiku for claude, or, for Codex and Cursor, a name from the agent's models in list_agents; leave it out to use the agent's default. Never guess a name: an unknown model fails the run."},
				"lock":     map[string]any{"type": "string", "maxLength": core.MaxTaskLock, "description": "optional name of a resource only one worker on that computer may use at a time, such as unity-editor; the task waits to start while an earlier unfinished task with the same lock is on that computer"},
			}, "op_id", "agent", "text")},
		{"name": "wait", "description": "Block until a new event of this project's tasks arrives after the cursor, or until timeout_seconds. Costs no tokens while blocked. Always pass after. Returns a bounded batch; more: true means further events are ready, so call wait again at once with the new cursor.",
			"inputSchema": obj(map[string]any{
				"after":           map[string]any{"type": "integer", "minimum": 0, "description": "cursor from the previous wait"},
				"timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": coordinatorMaxWait},
			})},
		{"name": "tasks", "description": "List this project's open tasks (not done, failed or cancelled), newest first, with any open question for you, the event cursor and a list version. state: \"all\" lists every task, or name one state. An open task shows the worker's last action and its time (last_activity, last_activity_at); every task shows the tokens it used so far. Pass if_version from the previous call to get unchanged: true instead of the same list again.",
			"inputSchema": obj(map[string]any{
				"state":      str("optional: \"all\", or one state such as running, waiting_coordinator, blocked, failed, done"),
				"limit":      map[string]any{"type": "integer", "minimum": 1, "maximum": coordinatorTaskMax, "description": fmt.Sprintf("most tasks to list; default %d", coordinatorTaskLimit)},
				"if_version": str("version from the previous tasks call"),
			})},
		{"name": "answer", "description": "Answer a worker's question. The worker continues in the same session. Repeating with the same op_id is safe.",
			"inputSchema": obj(map[string]any{
				"task_id":     str("task id"),
				"question_id": str("question id from the event"),
				"op_id":       str("unique id of this operation"),
				"text":        map[string]any{"type": "string", "minLength": 1, "maxLength": core.MaxTaskText},
			}, "task_id", "question_id", "op_id", "text")},
		{"name": "continue_task", "description": "Give a finished task (done or failed) of this computer a follow-up: the same worker conversation resumes with your text, so it keeps its context and needs no new briefing. If the agent no longer has that conversation, the run fails; then start a new task. Repeating with the same op_id is safe.",
			"inputSchema": obj(map[string]any{
				"task_id": str("task id"),
				"op_id":   str("unique id of this operation"),
				"text":    map[string]any{"type": "string", "minLength": 1, "maxLength": core.MaxTaskText},
			}, "task_id", "op_id", "text")},
		{"name": "cancel", "description": "Cancel a task: a waiting task at once, a running worker is stopped.",
			"inputSchema": obj(map[string]any{"task_id": str("task id")}, "task_id")},
		{"name": "get_log", "description": "Ask another computer of the network for the last lines of its Messenger log, to debug Messenger itself. It answers only if its owner allowed it. Waits up to 20 seconds; if the answer has not come, call again with request_id.",
			"inputSchema": obj(map[string]any{
				"computer":   str("id of another computer from list_agents"),
				"lines":      map[string]any{"type": "integer", "minimum": 1, "maximum": core.MaxLogLines, "description": fmt.Sprintf("how many last lines; default %d", core.DefaultLogLines)},
				"request_id": str("request_id from an earlier get_log that had no answer yet"),
			})},
	}
}

func (c *coordinator) call(ctx context.Context, params json.RawMessage) (any, error) {
	var p struct {
		Name string `json:"name"`
		Meta struct {
			ProgressToken json.RawMessage `json:"progressToken"`
		} `json:"_meta"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if len(p.Arguments) == 0 || string(p.Arguments) == "null" {
		p.Arguments = json.RawMessage(`{}`)
	}
	strict := func(target any) error {
		d := json.NewDecoder(strings.NewReader(string(p.Arguments)))
		d.DisallowUnknownFields()
		return d.Decode(target)
	}
	_ = c.store.TouchCoordinator(c.project)
	if err := c.projectExists(); err != nil {
		return nil, err
	}
	switch p.Name {
	case "list_agents":
		return c.store.ProjectAgents(c.project)
	case "start_task":
		var a struct {
			OpID     string `json:"op_id"`
			Agent    string `json:"agent"`
			Text     string `json:"text"`
			Title    string `json:"title"`
			Computer string `json:"computer"`
			Model    string `json:"model"`
			Lock     string `json:"lock"`
		}
		if err := strict(&a); err != nil {
			return nil, err
		}
		id, created, err := c.store.StartTask(core.StartTaskRequest{Project: c.project, OpID: a.OpID, Computer: a.Computer, Provider: a.Agent, Title: a.Title, Text: a.Text, Model: a.Model, Lock: a.Lock})
		if err != nil {
			return nil, err
		}
		return map[string]any{"task_id": id, "created": created, "next": "Call wait to receive its report."}, nil
	case "wait":
		var a struct {
			After   *int64 `json:"after"`
			Timeout int    `json:"timeout_seconds"`
		}
		if err := strict(&a); err != nil {
			return nil, err
		}
		after := int64(0)
		if a.After != nil {
			after = *a.After
		}
		timeout := a.Timeout
		if timeout <= 0 {
			timeout = coordinatorDefaultWait
		}
		if timeout > coordinatorMaxWait {
			timeout = coordinatorMaxWait
		}
		return c.wait(ctx, after, time.Duration(timeout)*time.Second, p.Meta.ProgressToken)
	case "tasks":
		var a struct {
			State     string `json:"state"`
			Limit     int    `json:"limit"`
			IfVersion string `json:"if_version"`
		}
		if err := strict(&a); err != nil {
			return nil, err
		}
		return c.tasks(a.State, a.Limit, a.IfVersion)
	case "answer":
		var a struct {
			TaskID     string `json:"task_id"`
			QuestionID string `json:"question_id"`
			OpID       string `json:"op_id"`
			Text       string `json:"text"`
		}
		if err := strict(&a); err != nil {
			return nil, err
		}
		if err := c.ownTask(a.TaskID); err != nil {
			return nil, err
		}
		if err := c.store.AnswerTask(a.TaskID, a.QuestionID, a.OpID, a.Text, core.AudienceCoordinator); err != nil {
			return nil, err
		}
		return map[string]any{"answered": true, "next": "The worker continues; call wait for its next report or question."}, nil
	case "continue_task":
		var a struct {
			TaskID string `json:"task_id"`
			OpID   string `json:"op_id"`
			Text   string `json:"text"`
		}
		if err := strict(&a); err != nil {
			return nil, err
		}
		if err := c.ownTask(a.TaskID); err != nil {
			return nil, err
		}
		if err := c.store.ContinueTask(a.TaskID, a.OpID, a.Text, core.AudienceCoordinator); err != nil {
			return nil, err
		}
		return map[string]any{"continued": true, "next": "The worker resumes its conversation; call wait for its report or question."}, nil
	case "cancel":
		var a struct {
			TaskID string `json:"task_id"`
		}
		if err := strict(&a); err != nil {
			return nil, err
		}
		if err := c.ownTask(a.TaskID); err != nil {
			return nil, err
		}
		state, err := c.store.CancelTask(a.TaskID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"state": state}, nil
	case "get_log":
		var a struct {
			Computer  string `json:"computer"`
			Lines     int    `json:"lines"`
			RequestID string `json:"request_id"`
		}
		if err := strict(&a); err != nil {
			return nil, err
		}
		return c.getLog(ctx, a.Computer, a.Lines, a.RequestID)
	}
	return nil, fmt.Errorf("unknown tool %q", p.Name)
}

// getLog asks another computer for its log, or looks up an earlier request,
// and waits a little for the answer.
func (c *coordinator) getLog(ctx context.Context, computer string, lines int, id string) (any, error) {
	if id == "" {
		if computer == "" {
			return nil, fmt.Errorf("%w: pass computer, or request_id from an earlier get_log", core.ErrRejected)
		}
		var err error
		if id, err = c.store.RequestNetLog(computer, lines); err != nil {
			return nil, err
		}
	}
	deadline := time.Now().Add(coordinatorLogWait)
	for {
		l, err := c.store.NetLog(id)
		if err != nil {
			return nil, err
		}
		if l.State != "waiting" || !time.Now().Before(deadline) {
			if l.State == "waiting" {
				return map[string]any{"request_id": l.ID, "computer": l.Computer, "state": l.State, "next": "No answer yet: that computer may be off or offline, or run a Messenger version without logs over the network. Call get_log again with request_id later."}, nil
			}
			return l, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(coordinatorPoll):
		}
	}
}

func (c *coordinator) projectExists() error {
	projects, err := c.store.Projects()
	if err != nil {
		return err
	}
	for _, p := range projects {
		if p.Name == c.project {
			return nil
		}
	}
	return fmt.Errorf("project %q does not exist; the owner creates it in the Messenger window (Новый проект…)", c.project)
}

// ownTask refuses tasks of another project.
func (c *coordinator) ownTask(id string) error {
	t, err := c.store.TaskByID(id)
	if err != nil {
		return err
	}
	if t.Project != c.project || t.FromNetwork {
		return errors.New("unknown task in this project")
	}
	return nil
}

type coordinatorTask struct {
	ID       string             `json:"task_id"`
	Title    string             `json:"title"`
	Agent    string             `json:"agent"`
	Computer string             `json:"computer"`
	State    string             `json:"state"`
	Detail   string             `json:"detail,omitempty"`
	Question *core.TaskQuestion `json:"question,omitempty"`
	// T-034: why a failed task failed, and when a usage limit resets.
	FailureReason string `json:"failure_reason,omitempty"`
	FailureReset  string `json:"failure_reset,omitempty"`
	CreatedAt     string `json:"created_at"`
	FinishedAt    string `json:"finished_at,omitempty"`
	// T-071: the worker's last action of an open task and when it was,
	// and the tokens the task used so far.
	LastActivity   string `json:"last_activity,omitempty"`
	LastActivityAt string `json:"last_activity_at,omitempty"`
	Tokens         int64  `json:"tokens,omitempty"`
	// T-072: the task waits to start until this task of the same lock ends.
	WaitingForLock string `json:"waiting_for_lock,omitempty"`
}

// closedTaskStates are left out of the default tasks list (T-032).
var closedTaskStates = map[string]bool{"done": true, "failed": true, "cancelled": true}

func (c *coordinator) tasks(state string, limit int, ifVersion string) (any, error) {
	if limit <= 0 {
		limit = coordinatorTaskLimit
	}
	if limit > coordinatorTaskMax {
		limit = coordinatorTaskMax
	}
	// Read the version first: a change after it shows up on the next call.
	version, err := c.store.TaskListVersion(c.project)
	if err != nil {
		return nil, err
	}
	cursor, err := c.store.LatestTaskEvent(c.project)
	if err != nil {
		return nil, err
	}
	if ifVersion != "" && ifVersion == version {
		return map[string]any{"unchanged": true, "version": version, "cursor": cursor}, nil
	}
	tasks, err := c.store.Tasks(c.project, coordinatorTaskMax)
	if err != nil {
		return nil, err
	}
	out := []coordinatorTask{}
	more := false
	for _, t := range tasks {
		// Another computer's coordinator manages the tasks it sent here.
		if t.FromNetwork {
			continue
		}
		switch state {
		case "":
			if closedTaskStates[t.State] {
				continue
			}
		case "all":
		default:
			if t.State != state {
				continue
			}
		}
		if len(out) == limit {
			more = true
			break
		}
		item := coordinatorTask{ID: t.ID, Title: t.Title, Agent: t.Provider, Computer: t.Computer, State: t.State, CreatedAt: t.CreatedAt, FinishedAt: t.FinishedAt, FailureReason: t.FailureReason, FailureReset: t.FailureReset}
		// A verified task's detail is the same verification line every
		// time; only unfinished or unsuccessful ones explain something.
		if t.State != "done" {
			item.Detail = t.Detail
		}
		if t.OpenQuestion != nil && t.OpenQuestion.Audience == core.AudienceCoordinator {
			item.Question = t.OpenQuestion
		}
		a, err := c.store.TaskActivity(t.ID)
		if err != nil {
			return nil, err
		}
		item.Tokens = a.Tokens
		if !closedTaskStates[t.State] {
			item.LastActivity, item.LastActivityAt = a.Last, a.At
			if item.WaitingForLock, err = c.store.TaskLockHolder(t.ID); err != nil {
				return nil, err
			}
		}
		out = append(out, item)
	}
	result := map[string]any{"tasks": out, "cursor": cursor, "version": version}
	if more {
		result["more"] = true
	}
	return result, nil
}

// boundEvents keeps events in order up to coordinatorWaitBudget characters of
// text, and always at least one (T-031). It reports whether any were left.
func boundEvents(events []core.TaskEvent) ([]core.TaskEvent, bool) {
	used := 0
	for i, e := range events {
		used += utf8.RuneCountInString(e.Text) + 200
		if i > 0 && used > coordinatorWaitBudget {
			return events[:i], true
		}
	}
	return events, false
}

func (c *coordinator) wait(ctx context.Context, after int64, timeout time.Duration, progressToken json.RawMessage) (any, error) {
	start := time.Now()
	hasProgress := len(progressToken) > 0 && string(progressToken) != "null"
	lastProgress := start
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(coordinatorPoll)
	defer tick.Stop()
	for {
		events, _, err := c.store.TaskEvents(c.project, core.AudienceCoordinator, after, coordinatorMaxReturned+1)
		if err != nil {
			return nil, err
		}
		if len(events) > 0 {
			more := len(events) > coordinatorMaxReturned
			if more {
				events = events[:coordinatorMaxReturned]
			}
			var trimmed bool
			events, trimmed = boundEvents(events)
			more = more || trimmed
			// The cursor is the last returned event, so nothing is skipped.
			cursor := events[len(events)-1].Seq
			result := map[string]any{"events": events, "cursor": cursor, "timed_out": false,
				"next": fmt.Sprintf("Act on these events, then call wait with after=%d.", cursor)}
			if more {
				result["more"] = true
				result["next"] = fmt.Sprintf("More events are ready. Act on these, then call wait with after=%d at once.", cursor)
			}
			return result, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return map[string]any{"events": []core.TaskEvent{}, "cursor": after, "timed_out": true,
				"next": fmt.Sprintf("Nothing yet. Call wait again with after=%d.", after)}, nil
		case <-tick.C:
			if hasProgress && time.Since(lastProgress) >= coordinatorProgress {
				lastProgress = time.Now()
				c.notify("notifications/progress", map[string]any{"progressToken": progressToken,
					"progress": int(time.Since(start).Seconds()), "message": "waiting for worker events"})
			}
		}
	}
}

func (c *coordinator) notify(method string, params any) {
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	c.out.Lock()
	defer c.out.Unlock()
	_, _ = c.w.Write(append(raw, '\n'))
}

func (c *coordinator) respond(id json.RawMessage, result any, rpcErr any) {
	msg := map[string]any{"jsonrpc": "2.0", "id": id}
	if rpcErr != nil {
		msg["error"] = rpcErr
	} else {
		msg["result"] = result
	}
	raw, _ := json.Marshal(msg)
	c.out.Lock()
	defer c.out.Unlock()
	_, _ = c.w.Write(append(raw, '\n'))
}

// CoordinatorMain is the entry for `messenger coordinator --project NAME`.
// The store is the client's data directory database; the server only writes
// task records and never launches a worker itself.
func CoordinatorMain(dbPath string, args []string) error {
	project := ""
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "--project" {
			project = args[i+1]
		}
	}
	if !core.ValidProjectName(project) {
		return errors.New("coordinator requires --project NAME")
	}
	return runCoordinator(dbPath, project, os.Getenv(adapters.WorkerRunVariable), os.Stdin, os.Stdout)
}

func runCoordinator(dbPath, project, workerRun string, in io.Reader, out io.Writer) error {
	if workerRun != "" {
		// Started by a worker that loaded the owner's coordinator connection
		// from the project folder: no tools, no database, no "connected" mark.
		return serveTask(nil, "", "", in, out)
	}
	store, err := core.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	return ServeCoordinator(store, project, in, out)
}
