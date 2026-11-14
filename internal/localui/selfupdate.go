package localui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/selfupdate"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

// SelfUpdater updates the program from the window on Windows and Linux
// (D-048). *selfupdate.Updater implements it.
type SelfUpdater interface {
	Status() selfupdate.Status
	Check(context.Context) selfupdate.Status
	Apply(context.Context) (selfupdate.Status, error)
}

// SelfUpdate is the window's part of the update: the updater and what to do
// once the new program is in place (close this page and start the new one).
type SelfUpdate struct {
	Updater  SelfUpdater
	Relaunch func()
}

// Desktop is what the window launcher enables besides the page itself.
type Desktop struct {
	setupRoot  string // Set only by RunWindow, or an isolated test.
	Channel    *updatefeed.Channel
	Restart    Restarter
	SelfUpdate *SelfUpdate
}

// selfUpdateDeadline covers running the new program's `version` (20 s at
// most) and writing it next to the old one.
const selfUpdateDeadline = 60 * time.Second

func selfUpdateStatus(u *SelfUpdate) map[string]any {
	s := u.Updater.Status()
	return map[string]any{"ok": true, "state": s.State, "version": s.Version, "error": s.Error, "installed_version": buildinfo.Version}
}

// serveSelfUpdate answers POST /api/self-update. "check" starts a check in
// the background and returns at once; "install" verifies the downloaded
// update again, puts it in place and then asks the launcher to restart the
// window. A failed install leaves the old program as it was.
func serveSelfUpdate(w http.ResponseWriter, r *http.Request, decoder *json.Decoder, u *SelfUpdate) {
	if u == nil || u.Updater == nil {
		http.NotFound(w, r)
		return
	}
	var input struct {
		Action string `json:"action"`
	}
	if err := decoder.Decode(&input); err != nil || decoder.Decode(new(any)) != io.EOF || (input.Action != "check" && input.Action != "install") {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if input.Action == "check" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			u.Updater.Check(ctx)
		}()
		writeJSON(w, map[string]any{"ok": true, "state": "checking"})
		return
	}
	s, err := u.Updater.Apply(r.Context())
	if err != nil {
		reject(w, http.StatusConflict, s.State, "Обновление не установлено: "+err.Error()+". Программа осталась прежней.")
		return
	}
	writeJSON(w, map[string]any{"ok": true, "state": s.State, "version": s.Version})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	if u.Relaunch != nil {
		// Let the answer reach the page before it closes.
		time.AfterFunc(time.Second, u.Relaunch)
	}
}
