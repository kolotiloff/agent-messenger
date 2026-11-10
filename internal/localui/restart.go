package localui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/control"
)

// Restarter stops the background client and starts this build's client in
// its place. Only the desktop launcher provides one: it knows how the window
// starts a client. A browser window gets none and shows a command instead.
type Restarter func(context.Context) error

var releaseVersion = regexp.MustCompile(`^([0-9]+)\.([0-9]+)\.([0-9]+)-preview\.([0-9]+)$`)

// clientOutdated reports whether the background client is older than this
// window. A client without a version predates version reporting. Two
// versions that cannot be compared are never called outdated, so the window
// never offers to replace a newer client with an older one.
func clientOutdated(client, window string) bool {
	if client == window {
		return false
	}
	if client == "" {
		return true
	}
	c, w := releaseVersion.FindStringSubmatch(client), releaseVersion.FindStringSubmatch(window)
	if c == nil || w == nil {
		return false
	}
	for i := 1; i < len(c); i++ {
		a, _ := strconv.Atoi(c[i])
		b, _ := strconv.Atoi(w[i])
		if a != b {
			return a < b
		}
	}
	return false
}

// clientInfo answers which client version runs and whether the window can
// restart it. It reads only the client's status.
func clientInfo(ctx context.Context, call caller, restart Restarter) map[string]any {
	info := map[string]any{"window_version": buildinfo.Version, "restart": restart != nil}
	response, err := call(ctx, control.Request{Cmd: "status"})
	if err != nil || !response.OK || response.Status == nil {
		info["client"] = "unavailable"
		return info
	}
	info["client"] = "running"
	info["client_version"] = response.Status.Version
	if response.Status.Sandbox != "" {
		info["sandbox"] = response.Status.Sandbox
	}
	info["outdated"] = clientOutdated(response.Status.Version, buildinfo.Version)
	return info
}

// runningTask is one task the restart would cut, as the window lists it.
type runningTask struct {
	Agent     string `json:"agent"`
	Title     string `json:"title"`
	StartedAt string `json:"started_at,omitempty"`
}

// runningTasks counts task runs on this computer from the window view. A
// client that cannot answer makes the count unknown, never zero. The list
// holds the running tasks of the project the window shows (T-123); the
// count covers every project.
func runningTasks(ctx context.Context, call caller, project string) (int, bool, []runningTask) {
	response, err := call(ctx, control.Request{Cmd: "window-view", Alias: project})
	if err != nil || !response.OK || len(response.Overview) == 0 {
		return 0, false, nil
	}
	var view struct {
		Agents *[]struct {
			Running int `json:"running"`
		} `json:"agents"`
		Tasks []struct {
			Agent         string `json:"agent"`
			Title         string `json:"title"`
			State         string `json:"state"`
			StartedAt     string `json:"started_at"`
			CreatedAt     string `json:"created_at"`
			PendingLaunch bool   `json:"pending_launch"`
			Remote        *struct {
				From bool `json:"from"`
			} `json:"remote"`
		} `json:"tasks"`
	}
	if json.Unmarshal(response.Overview, &view) != nil || view.Agents == nil {
		return 0, false, nil
	}
	n := 0
	for _, a := range *view.Agents {
		n += a.Running
	}
	list := []runningTask{}
	for _, t := range view.Tasks {
		// A task this computer sent elsewhere runs there, not here.
		if t.State != "running" || t.PendingLaunch || (t.Remote != nil && !t.Remote.From) {
			continue
		}
		at := t.StartedAt
		if at == "" {
			at = t.CreatedAt
		}
		list = append(list, runningTask{Agent: t.Agent, Title: t.Title, StartedAt: at})
	}
	return n, true, list
}

func serveRestart(w http.ResponseWriter, r *http.Request, decoder *json.Decoder, call caller, restart Restarter, busy chan struct{}) {
	if restart == nil {
		http.NotFound(w, r)
		return
	}
	var input struct {
		Project string `json:"project"`
		Confirm bool   `json:"confirm"`
	}
	if err := decoder.Decode(&input); err != nil || decoder.Decode(new(any)) != io.EOF || len(input.Project) > 256 {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	select {
	case busy <- struct{}{}:
		defer func() { <-busy }()
	default:
		reject(w, http.StatusConflict, "busy", "Перезапуск уже идёт.")
		return
	}
	ctx := r.Context()
	response, err := call(ctx, control.Request{Cmd: "status"})
	if err != nil || !response.OK || response.Status == nil {
		reject(w, http.StatusServiceUnavailable, "unavailable", "Фоновый клиент не ответил. Ничего не остановлено.")
		return
	}
	if !clientOutdated(response.Status.Version, buildinfo.Version) {
		reject(w, http.StatusConflict, "current", "Фоновый клиент уже этой версии. Перезапуск не нужен.")
		return
	}
	// A task run interrupted by the restart stays blocked until the owner
	// decides; it is never repeated by itself. Ask before doing that.
	running, known, tasks := runningTasks(ctx, call, input.Project)
	if !input.Confirm && (!known || running > 0) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, map[string]any{"ok": false, "state": "confirm", "running": running, "known": known, "tasks": tasks})
		return
	}
	if err := restart(ctx); err != nil {
		reject(w, http.StatusBadGateway, "failed", err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true, "action": "restart-client"})
}

func reject(w http.ResponseWriter, code int, state, detail string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	writeJSON(w, map[string]any{"ok": false, "state": state, "detail": detail})
}
