package localui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
)

const taskFileRoute = "/api/task-file"

func serveTaskFile(w http.ResponseWriter, r *http.Request, call caller) {
	var input struct {
		Task   string `json:"task"`
		Path   string `json:"path"`
		Action string `json:"action"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&input) != nil || dec.Decode(new(any)) != io.EOF || input.Task == "" || len(input.Task) > 128 ||
		input.Path == "" || len(input.Path) > 4096 || strings.ContainsAny(input.Path, "\x00\r\n") ||
		(input.Action != "preview" && input.Action != "download" && input.Action != "reveal" && input.Action != "edit") {
		http.Error(w, "invalid file action", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	response, err := call(ctx, control.Request{Cmd: "task-file", MessageID: input.Task, Workspace: input.Path, Action: input.Action})
	if err != nil {
		http.Error(w, "local client unavailable", http.StatusServiceUnavailable)
		return
	}
	if !response.OK {
		if response.Error == "неизвестная команда" {
			http.Error(w, "client_outdated", http.StatusBadGateway)
			return
		}
		http.Error(w, "file unavailable", http.StatusNotFound)
		return
	}
	writeRaw(w, response.Overview)
}
