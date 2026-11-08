// Package localui exposes selected local reads and explicit queue actions.
// Explicit diagnostics execute only the selected CLI's qualified status
// commands. The UI never opens model sessions or proxies arbitrary commands.
package localui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/diagnostics"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

//go:embed assets/*
var assets embed.FS

// AppIcon is the program's icon (the Mac app's drawing) as SVG, for the
// Linux application menu.
func AppIcon() []byte {
	data, _ := assets.ReadFile("assets/icon.svg")
	return data
}

type caller func(context.Context, control.Request) (control.Response, error)

// Run binds only IPv4 loopback, with a fresh capability for each invocation.
// The URL is private. The caller must not publish or log it to a shared service.
func Run(ctx context.Context, root string, port int, out io.Writer) error {
	return RunDesktop(ctx, root, port, out, nil, nil)
}

// RunDesktop serves the native desktop window. Updates and the client
// restart are enabled only by the desktop launcher, never by a browser UI.
func RunDesktop(ctx context.Context, root string, port int, out io.Writer, channel *updatefeed.Channel, restart Restarter) error {
	return RunWindow(ctx, root, port, out, Desktop{Channel: channel, Restart: restart})
}

// RunWindow serves the window with what its launcher enables.
func RunWindow(ctx context.Context, root string, port int, out io.Writer, desktop Desktop) error {
	if port < 0 || port > 65535 {
		return errors.New("ui port must be 0..65535")
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	token := hex.EncodeToString(secret)
	ln, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return err
	}
	defer ln.Close()
	host := ln.Addr().String()
	call := func(ctx context.Context, r control.Request) (control.Response, error) {
		return control.CallContext(ctx, root, r)
	}
	h := newDesktopHandler(host, token, call, func(ctx context.Context) any {
		r := diagnostics.CollectDesktop(ctx, root)
		// Only selected installation/profile paths reach this authenticated local
		// setup UI; never credentials, session IDs or the database inspection.
		type candidate struct {
			Provider    string `json:"provider"`
			Name        string `json:"name"`
			State       string `json:"state"`
			CLI         string `json:"cli,omitempty"`
			Environment string `json:"environment,omitempty"`
		}
		list := []candidate{}
		home, _ := os.UserHomeDir()
		for _, a := range r.Agents {
			profile := ""
			if home != "" {
				profile = filepath.Join(home, "."+a.Provider)
			}
			list = append(list, candidate{a.Provider, a.Name, a.CLI.State, a.CLI.Path, profile})
		}
		return struct {
			OS        string      `json:"os"`
			CheckedAt string      `json:"checked_at"`
			Agents    []candidate `json:"agents"`
		}{r.OS, r.CheckedAt, list}
	}, diagnostics.CheckCLI, desktop)
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 20 * time.Second, MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0)}
	stop := context.AfterFunc(ctx, func() { _ = srv.Close() })
	defer stop()
	if _, err := fmt.Fprintf(out, "http://%s/#%s\n", host, token); err != nil {
		return err
	}
	err = srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func newHandler(host, token string, call caller, discover func(context.Context) any) http.Handler {
	return newHandlerWithUpdates(host, token, call, discover, nil)
}

func newHandlerWithUpdates(host, token string, call caller, discover func(context.Context) any, channel *updatefeed.Channel) http.Handler {
	return newHandlerWithChecks(host, token, call, discover, channel, diagnostics.CheckCLI, nil)
}

func newHandlerWithChecks(host, token string, call caller, discover func(context.Context) any, channel *updatefeed.Channel, checker cliChecker, restart Restarter) http.Handler {
	return newDesktopHandler(host, token, call, discover, checker, Desktop{Channel: channel, Restart: restart})
}

func newDesktopHandler(host, token string, call caller, discover func(context.Context) any, checker cliChecker, desktop Desktop) http.Handler {
	channel, restart, self := desktop.Channel, desktop.Restart, desktop.SelfUpdate
	busy := make(chan struct{}, 1)
	restarting := make(chan struct{}, 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		if r.Host != host {
			http.Error(w, "invalid host", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+host {
			http.Error(w, "invalid origin", http.StatusForbidden)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" && !startPageNavigation(r) {
			http.Error(w, "cross-site request refused", http.StatusForbidden)
			return
		}
		mutation := r.URL.Path == "/api/enqueue" || r.URL.Path == "/api/pause" || r.URL.Path == "/api/resume" || r.URL.Path == "/api/outbox/ack" || r.URL.Path == "/api/check-cli" || r.URL.Path == "/api/setup-claude" || r.URL.Path == "/api/setup-codex" || r.URL.Path == "/api/authorize-setup" || r.URL.Path == "/api/restart-client" || r.URL.Path == "/api/pick" || r.URL.Path == "/api/agent-login" || r.URL.Path == "/api/agent-logout" || r.URL.Path == "/api/agent-install" || taskMutation(r.URL.Path) ||
			(r.URL.Path == "/api/self-update" && r.Method != http.MethodGet)
		method := http.MethodGet
		if mutation {
			method = http.MethodPost
		}
		if r.Method != method {
			w.Header().Set("Allow", method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			files := map[string]string{"/": "index.html", "/app.js": "app.js", "/app.css": "app.css", "/design-v1.css": "design-v1.css", "/icon.svg": "icon.svg"}
			name, ok := files[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			data, err := assets.ReadFile("assets/" + name)
			if err != nil {
				http.Error(w, "asset unavailable", 500)
				return
			}
			switch {
			case strings.HasSuffix(name, ".css"):
				w.Header().Set("Content-Type", "text/css; charset=utf-8")
			case strings.HasSuffix(name, ".js"):
				w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			case strings.HasSuffix(name, ".svg"):
				w.Header().Set("Content-Type", "image/svg+xml")
			default:
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
			}
			_, _ = w.Write(data)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "access denied", http.StatusUnauthorized)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/updates/") {
			if channel == nil {
				http.NotFound(w, r)
				return
			}
			channel.ServeHTTP(w, r)
			return
		}
		deadline := 3 * time.Second
		if r.URL.Path == "/api/check-cli" {
			deadline = 20 * time.Second
		}
		if r.URL.Path == "/api/server-action" {
			// Joining or checking a server goes over the network.
			deadline = 22 * time.Second
		}
		if r.URL.Path == "/api/setup-codex" {
			// Starting app-server and creating a thread takes a few RPC rounds.
			deadline = 100 * time.Second
		}
		if r.URL.Path == "/api/self-update" {
			if r.Method == http.MethodGet {
				if self == nil || self.Updater == nil {
					http.NotFound(w, r)
					return
				}
				writeJSON(w, selfUpdateStatus(self))
				return
			}
			deadline = selfUpdateDeadline
		}
		if r.URL.Path == "/api/coordinator-connect" {
			// One `claude mcp add` run, bounded at 30 s by the client.
			deadline = 45 * time.Second
		}
		if r.URL.Path == "/api/restart-client" {
			// Waits for the old client to exit and the new one to confirm.
			deadline = 45 * time.Second
		}
		if r.URL.Path == "/api/pick" {
			// The owner chooses in the system's own dialog.
			deadline = pickerDeadline
		}
		// A long route must still be able to write its answer.
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(deadline + 3*time.Second))
		ctx, cancel := context.WithTimeout(r.Context(), deadline)
		defer cancel()
		if mutation {
			// A key alone is not enough for a browser mutation: require the exact
			// origin and a JSON envelope, with bounded size and no extra fields.
			if r.Header.Get("Origin") != "http://"+host {
				http.Error(w, "origin required", 403)
				return
			}
			media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || media != "application/json" {
				http.Error(w, "JSON required", 415)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if r.URL.Path == "/api/check-cli" {
				serveCLICheck(w, r.WithContext(ctx), decoder, checker, busy)
				return
			}
			if r.URL.Path == "/api/self-update" {
				serveSelfUpdate(w, r.WithContext(ctx), decoder, self)
				return
			}
			if r.URL.Path == "/api/pick" {
				servePick(w, r.WithContext(ctx), decoder, pickFunc)
				return
			}
			if r.URL.Path == "/api/agent-login" || r.URL.Path == "/api/agent-logout" {
				serveAgentLogin(w, decoder, r.URL.Path == "/api/agent-logout")
				return
			}
			if r.URL.Path == "/api/agent-install" {
				serveAgentInstall(w, decoder)
				return
			}
			if r.URL.Path == "/api/restart-client" {
				serveRestart(w, r.WithContext(ctx), decoder, call, restart, restarting)
				return
			}
			var req control.Request
			if taskMutation(r.URL.Path) {
				var ok bool
				if req, ok = decodeTaskMutation(r.URL.Path, decoder); !ok {
					http.Error(w, "invalid request", 400)
					return
				}
				if decoder.Decode(new(any)) != io.EOF {
					http.Error(w, "one JSON object required", 400)
					return
				}
				response, err := call(ctx, req)
				if err != nil {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusServiceUnavailable)
					writeJSON(w, map[string]any{"ok": false, "state": "unconfirmed"})
					return
				}
				if !response.OK && response.Error == "неизвестная команда" {
					// The running client is older than this window.
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusBadGateway)
					writeJSON(w, map[string]any{"ok": false, "state": "client_outdated"})
					return
				}
				if !response.OK {
					// Task errors are plain rule messages; they carry no secrets.
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusConflict)
					writeJSON(w, map[string]any{"ok": false, "state": "rejected", "detail": response.Error})
					return
				}
				if len(response.Overview) > 0 {
					writeJSON(w, map[string]any{"ok": true, "action": req.Cmd, "result": response.Overview})
					return
				}
				writeJSON(w, map[string]any{"ok": true, "action": req.Cmd})
				return
			}
			if r.URL.Path == "/api/authorize-setup" {
				var input struct {
					Alias        string `json:"alias"`
					Confirmation string `json:"confirmation"`
				}
				if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.Alias) == "" || len(input.Alias) > 128 || input.Confirmation != "allow-model-calls" {
					http.Error(w, "explicit test-session authorization required", 400)
					return
				}
				req = control.Request{Cmd: "authorize-setup", Alias: input.Alias, Confirmation: input.Confirmation}
			} else if r.URL.Path == "/api/setup-claude" {
				var input struct {
					ID          string `json:"id"`
					Alias       string `json:"alias"`
					CLI         string `json:"cli"`
					Environment string `json:"environment"`
				}
				if err := decoder.Decode(&input); err != nil || !core.ValidSetupID(input.ID) || strings.TrimSpace(input.Alias) == "" || len(input.Alias) > 128 || !filepath.IsAbs(input.CLI) || !filepath.IsAbs(input.Environment) || len(input.CLI) > 4096 || len(input.Environment) > 4096 {
					http.Error(w, "invalid new session", 400)
					return
				}
				req = control.Request{Cmd: "setup-new-claude", ConversationID: input.ID, Alias: input.Alias, CLI: input.CLI, Environment: input.Environment}
			} else if r.URL.Path == "/api/setup-codex" {
				var input struct {
					Alias        string `json:"alias"`
					CLI          string `json:"cli"`
					Environment  string `json:"environment"`
					Confirmation string `json:"confirmation"`
				}
				if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.Alias) == "" || len(input.Alias) > 128 || !filepath.IsAbs(input.CLI) || !filepath.IsAbs(input.Environment) || len(input.CLI) > 4096 || len(input.Environment) > 4096 || input.Confirmation != "allow-model-calls" {
					http.Error(w, "invalid Codex session", 400)
					return
				}
				req = control.Request{Cmd: "setup-codex", Alias: input.Alias, CLI: input.CLI, Environment: input.Environment, Confirmation: input.Confirmation}
			} else if r.URL.Path == "/api/enqueue" {
				var envelope struct {
					ID    string `json:"message_id"`
					Alias string `json:"alias"`
					Text  string `json:"text"`
				}
				if err := decoder.Decode(&envelope); err != nil {
					http.Error(w, "invalid message", 400)
					return
				}
				if !core.ValidUIMessageID(envelope.ID) || strings.TrimSpace(envelope.Alias) == "" || len(envelope.Alias) > 4096 || strings.TrimSpace(envelope.Text) == "" || len(envelope.Text) > 16000 {
					http.Error(w, "invalid message", 400)
					return
				}
				req = control.Request{Cmd: "ui-prepare", MessageID: envelope.ID, Alias: envelope.Alias, Text: envelope.Text, Sender: "local-user"}
			} else if r.URL.Path == "/api/outbox/ack" {
				var envelope struct {
					ID string `json:"message_id"`
				}
				if err := decoder.Decode(&envelope); err != nil || !core.ValidUIMessageID(envelope.ID) {
					http.Error(w, "invalid acknowledgment", 400)
					return
				}
				req = control.Request{Cmd: "ui-ack", MessageID: envelope.ID}
			} else {
				var envelope *struct{}
				if err := decoder.Decode(&envelope); err != nil || envelope == nil {
					http.Error(w, "invalid command", 400)
					return
				}
				req.Cmd = strings.TrimPrefix(r.URL.Path, "/api/")
			}
			if decoder.Decode(new(any)) != io.EOF {
				http.Error(w, "one JSON object required", 400)
				return
			}
			response, err := call(ctx, req)
			if req.Cmd == "setup-codex" && err == nil && !response.OK {
				// Setup errors carry no secrets: no config values, server names or tokens.
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusConflict)
				writeJSON(w, map[string]any{"ok": false, "state": "rejected", "detail": response.Error})
				return
			}
			if (req.Cmd == "setup-new-claude" || req.Cmd == "authorize-setup") && err == nil && !response.OK {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusConflict)
				writeJSON(w, map[string]any{"ok": false, "state": "rejected"})
				return
			}
			if req.Cmd == "ui-prepare" && err == nil && response.OK {
				req = control.Request{Cmd: "ui-submit", MessageID: req.MessageID}
				response, err = call(ctx, req)
			}
			if err != nil || !response.OK || (req.Cmd == "ui-submit" && response.Created == nil) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusServiceUnavailable)
				writeJSON(w, map[string]any{"ok": false, "state": "unconfirmed", "message_id": req.MessageID})
				return
			}
			// This ACK proves queue persistence only, never provider success.
			if req.Cmd == "ui-submit" {
				writeJSON(w, map[string]any{"ok": true, "message_id": req.MessageID, "created": *response.Created})
				return
			}
			if req.Cmd == "setup-codex" {
				writeJSON(w, map[string]any{"ok": true, "action": req.Cmd, "alias": req.Alias})
				return
			}
			if req.Cmd == "setup-new-claude" {
				writeJSON(w, map[string]any{"ok": true, "action": req.Cmd, "id": req.ConversationID, "alias": req.Alias})
				return
			}
			writeJSON(w, map[string]any{"ok": true, "action": req.Cmd})
			return
		}
		if r.URL.Path == "/api/discovery" {
			writeJSON(w, discover(ctx))
			return
		}
		if r.URL.Path == "/api/client-info" {
			writeJSON(w, clientInfo(ctx, call, restart))
			return
		}
		req := control.Request{}
		switch r.URL.Path {
		case "/api/outbox":
			req.Cmd = "ui-outbox"
		case "/api/status":
			req.Cmd = "status"
		case "/api/overview":
			req.Cmd = "overview"
		case "/api/server":
			req.Cmd, req.Action = "server", "status"
		case "/api/window":
			req.Cmd = "window-view"
			req.Alias = r.URL.Query().Get("project")
			if len(req.Alias) > 256 {
				http.Error(w, "invalid project", 400)
				return
			}
		case "/api/task":
			req.Cmd = "task-feed"
			req.MessageID = r.URL.Query().Get("id")
			if req.MessageID == "" || len(req.MessageID) > 128 {
				http.Error(w, "invalid task", 400)
				return
			}
		case "/api/timeline":
			req.Cmd = "timeline"
			req.Alias = r.URL.Query().Get("alias")
			req.Before = r.URL.Query().Get("before")
			req.Limit = 50
			if v := r.URL.Query().Get("limit"); v != "" {
				n, err := strconv.Atoi(v)
				if err != nil {
					http.Error(w, "invalid limit", 400)
					return
				}
				req.Limit = n
			}
			if strings.TrimSpace(req.Alias) == "" || len(req.Alias) > 4096 || len(req.Before) > 4096 || req.Limit < 1 || req.Limit > 100 {
				http.Error(w, "invalid page", 400)
				return
			}
		default:
			http.NotFound(w, r)
			return
		}
		response, err := call(ctx, req)
		if err != nil {
			http.Error(w, "client_unavailable: local client unavailable or request rejected", http.StatusServiceUnavailable)
			return
		}
		if !response.OK {
			// Only the kind of failure is shown; the client's own detail
			// may name private paths.
			kind := "client_rejected"
			if response.Error == "неизвестная команда" {
				kind = "client_outdated"
			}
			http.Error(w, kind+": the local client did not answer this request", http.StatusBadGateway)
			return
		}
		switch req.Cmd {
		case "ui-outbox":
			writeRaw(w, response.Outbox)
		case "status":
			if response.Status == nil {
				http.Error(w, "missing status", 502)
				return
			}
			status := *response.Status
			status.Detail = ""
			writeJSON(w, status)
		case "overview", "window-view", "task-feed", "server":
			writeRaw(w, response.Overview)
		case "timeline":
			writeRaw(w, response.Timeline)
		}
	})
}

func writeRaw(w http.ResponseWriter, raw json.RawMessage) {
	if len(raw) == 0 || !json.Valid(raw) {
		http.Error(w, "invalid client response", 502)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(raw)
}

func writeJSON(w http.ResponseWriter, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "invalid response", 500)
		return
	}
	writeRaw(w, raw)
}

// serverActions are the window's server settings actions.
var serverActions = map[string]bool{"enable": true, "disable": true, "invite": true, "revoke": true, "connect": true, "check": true, "disconnect": true, "accept": true, "rename": true}

func taskMutation(path string) bool {
	switch path {
	case "/api/project-create", "/api/project-folder", "/api/task-answer", "/api/agent-set", "/api/agent-models", "/api/agent-access", "/api/agent-pause", "/api/agent-remove", "/api/task-cancel", "/api/server-action", "/api/coordinator-connect", "/api/project-rules", "/api/project-title", "/api/project-remove":
		return true
	}
	return false
}

// decodeTaskMutation reads one window action of the task model. The client
// re-checks every rule; this only bounds the input.
func decodeTaskMutation(path string, decoder *json.Decoder) (control.Request, bool) {
	switch path {
	case "/api/project-create", "/api/project-folder":
		var in struct {
			Name   string `json:"name"`
			Folder string `json:"folder"`
		}
		if decoder.Decode(&in) != nil || !core.ValidProjectName(in.Name) || !filepath.IsAbs(in.Folder) || len(in.Folder) > 4096 {
			return control.Request{}, false
		}
		cmd := strings.TrimPrefix(path, "/api/")
		return control.Request{Cmd: cmd, Alias: in.Name, Workspace: in.Folder}, true
	case "/api/project-rules":
		var in struct {
			Name  string `json:"name"`
			Rules string `json:"rules"`
		}
		if decoder.Decode(&in) != nil || !core.ValidProjectName(in.Name) || len(in.Rules) > 4*core.MaxWorkerRules {
			return control.Request{}, false
		}
		return control.Request{Cmd: "project-rules", Alias: in.Name, Text: in.Rules}, true
	case "/api/project-title":
		var in struct {
			Name  string `json:"name"`
			Title string `json:"title"`
		}
		if decoder.Decode(&in) != nil || !core.ValidProjectName(in.Name) || len(in.Title) > 1024 {
			return control.Request{}, false
		}
		return control.Request{Cmd: "project-title", Alias: in.Name, Text: in.Title}, true
	case "/api/project-remove":
		var in struct {
			Name string `json:"name"`
		}
		if decoder.Decode(&in) != nil || !core.ValidProjectName(in.Name) {
			return control.Request{}, false
		}
		return control.Request{Cmd: "project-remove", Alias: in.Name}, true
	case "/api/coordinator-connect":
		var in struct {
			Project string `json:"project"`
		}
		if decoder.Decode(&in) != nil || !core.ValidProjectName(in.Project) {
			return control.Request{}, false
		}
		return control.Request{Cmd: "coordinator-connect", Alias: in.Project}, true
	case "/api/task-answer":
		var in struct {
			Task     string `json:"task"`
			Question string `json:"question"`
			OpID     string `json:"op_id"`
			Text     string `json:"text"`
		}
		if decoder.Decode(&in) != nil || in.Task == "" || len(in.Task) > 128 || in.Question == "" || len(in.Question) > 128 || in.OpID == "" || len(in.OpID) > 128 || strings.TrimSpace(in.Text) == "" || len(in.Text) > 4*core.MaxTaskText {
			return control.Request{}, false
		}
		return control.Request{Cmd: "task-owner-answer", MessageID: in.Task, ConversationID: in.Question, Confirmation: in.OpID, Text: in.Text}, true
	case "/api/server-action":
		var in struct {
			Action  string `json:"action,omitempty"`
			Scope   string `json:"scope,omitempty"`
			Port    int    `json:"port,omitempty"`
			Address string `json:"address,omitempty"`
			Device  string `json:"device,omitempty"`
			Code    string `json:"code,omitempty"`
			Name    string `json:"name,omitempty"`
			On      bool   `json:"on,omitempty"`
		}
		if decoder.Decode(&in) != nil || !serverActions[in.Action] || len(in.Scope) > 16 || in.Port < 0 || in.Port > 65535 || len(in.Address) > 255 || len(in.Device) > 64 || len(in.Code) > 2048 || len(in.Name) > 256 {
			return control.Request{}, false
		}
		action := in.Action
		in.Action = ""
		payload, _ := json.Marshal(in)
		return control.Request{Cmd: "server", Action: action, Text: string(payload)}, true
	case "/api/task-cancel":
		var in struct {
			Task string `json:"task"`
		}
		if decoder.Decode(&in) != nil || in.Task == "" || len(in.Task) > 128 {
			return control.Request{}, false
		}
		return control.Request{Cmd: "task-cancel", MessageID: in.Task}, true
	case "/api/agent-set":
		var in struct {
			Provider    string `json:"provider"`
			CLI         string `json:"cli"`
			Environment string `json:"environment"`
		}
		if decoder.Decode(&in) != nil || !core.TaskProviders[in.Provider] || !filepath.IsAbs(in.CLI) || len(in.CLI) > 4096 || (in.Environment != "" && !filepath.IsAbs(in.Environment)) || len(in.Environment) > 4096 {
			return control.Request{}, false
		}
		return control.Request{Cmd: "agent-set", Provider: in.Provider, CLI: in.CLI, Environment: in.Environment}, true
	case "/api/agent-remove":
		var in struct {
			Provider string `json:"provider"`
		}
		if decoder.Decode(&in) != nil || !core.TaskProviders[in.Provider] {
			return control.Request{}, false
		}
		return control.Request{Cmd: "agent-remove", Provider: in.Provider}, true
	case "/api/agent-access":
		var in struct {
			Provider string `json:"provider"`
			On       *bool  `json:"on"`
		}
		// "all" is the one switch for every agent of this computer (T-056).
		if decoder.Decode(&in) != nil || (!core.TaskProviders[in.Provider] && in.Provider != "all") || in.On == nil {
			return control.Request{}, false
		}
		text := "0"
		if *in.On {
			text = "1"
		}
		return control.Request{Cmd: "agent-access", Provider: in.Provider, Text: text}, true
	case "/api/agent-pause":
		var in struct {
			Provider string `json:"provider"`
			Paused   *bool  `json:"paused"`
		}
		if decoder.Decode(&in) != nil || !core.TaskProviders[in.Provider] || in.Paused == nil {
			return control.Request{}, false
		}
		text := "0"
		if *in.Paused {
			text = "1"
		}
		return control.Request{Cmd: "agent-pause", Provider: in.Provider, Text: text}, true
	case "/api/agent-models":
		var in struct {
			Provider string   `json:"provider"`
			Models   []string `json:"models"`
		}
		if decoder.Decode(&in) != nil || !core.TaskProviders[in.Provider] || len(in.Models) > core.MaxAgentModels {
			return control.Request{}, false
		}
		for _, m := range in.Models {
			if strings.ContainsAny(m, "\n\r") || !core.ValidTaskModel(strings.TrimSpace(m)) {
				return control.Request{}, false
			}
		}
		return control.Request{Cmd: "agent-models", Provider: in.Provider, Text: strings.Join(in.Models, "\n")}, true
	}
	return control.Request{}, false
}

// startPageNavigation is the browser opening the start page from the
// forwarding file of "messenger open" (a file:// page, so the browser calls
// it cross-site). The start page holds no data and the key stays in the
// address fragment, which is never sent; every API call still needs the same
// origin and the key.
func startPageNavigation(r *http.Request) bool {
	return r.Method == http.MethodGet && r.URL.Path == "/" && r.URL.RawQuery == "" &&
		r.Header.Get("Sec-Fetch-Mode") == "navigate" && r.Header.Get("Sec-Fetch-Dest") == "document"
}
