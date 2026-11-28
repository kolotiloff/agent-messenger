package localui

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

const sleepModeRoute = "/api/sleep-mode"
const onboardingRoute = "/api/onboarding"

func serveWindowPreference(w http.ResponseWriter, r *http.Request, root string, call caller, decoder *json.Decoder) {
	if r.URL.Path == onboardingRoute && root == "" {
		http.NotFound(w, r)
		return
	}
	var payload struct {
		Mode   string `json:"mode"`
		Action string `json:"action"`
		Model  string `json:"model"`
	}
	if decoder != nil {
		if err := decoder.Decode(&payload); err != nil || decoder.Decode(new(any)) != io.EOF {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
	}
	switch r.URL.Path {
	case sleepModeRoute:
		if decoder != nil && (payload.Action != "" || payload.Model != "" || payload.Mode != "off" && payload.Mode != "working" && payload.Mode != "always") {
			http.Error(w, "invalid sleep mode", http.StatusBadRequest)
			return
		}
		response, err := call(r.Context(), control.Request{Cmd: "sleep-mode", Action: payload.Mode})
		if err != nil || !response.OK || len(response.Overview) == 0 {
			reject(w, http.StatusServiceUnavailable, "unconfirmed", "Режим сна недоступен.")
			return
		}
		writeJSON(w, response.Overview)
	case onboardingRoute:
		if decoder == nil {
			writeJSON(w, map[string]any{"pending": platform.WindowSetupPending(root), "model": platform.WindowDefaultModel(root)})
			return
		}
		if payload.Mode != "" || (payload.Action != "default-model" && payload.Model != "") {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		switch payload.Action {
		case "dismiss":
			if err := platform.DismissWindowSetup(root); err != nil {
				reject(w, http.StatusServiceUnavailable, "unconfirmed", "Не удалось сохранить настройку.")
				return
			}
			writeJSON(w, map[string]bool{"ok": true})
		case "default-model":
			provider, model, valid := strings.Cut(payload.Model, ":")
			if !valid || (provider != "claude" && provider != "codex" && provider != "cursor") ||
				!core.ValidTaskModel(model) {
				http.Error(w, "invalid default model", http.StatusBadRequest)
				return
			}
			if err := platform.SetWindowDefaultModel(root, payload.Model); err != nil {
				reject(w, http.StatusServiceUnavailable, "unconfirmed", "Не удалось сохранить настройку.")
				return
			}
			writeJSON(w, map[string]bool{"ok": true})
		case "direct-chat":
			// Workers require an executable working directory. Give a local
			// chat its own empty working directory without asking for a project.
			folder := filepath.Join(root, "local-chat")
			if err := os.MkdirAll(folder, 0o700); err != nil {
				reject(w, http.StatusServiceUnavailable, "unconfirmed", "Не удалось подготовить папку чата.")
				return
			}
			const project = "Личный чат"
			response, err := call(r.Context(), control.Request{Cmd: "project-create", Alias: project, Workspace: folder})
			if err != nil || !response.OK {
				reject(w, http.StatusServiceUnavailable, "unconfirmed", "Не удалось подготовить личный чат.")
				return
			}
			writeJSON(w, map[string]any{"ok": true, "project": project})
		default:
			http.Error(w, "invalid request", http.StatusBadRequest)
		}
	}
}
