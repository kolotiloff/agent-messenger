package localui

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

func TestUpdateProgress(t *testing.T) {
	channel := &updatefeed.Channel{Directory: t.TempDir()}
	h := newHandlerWithUpdates("127.0.0.1:1234", "secret", func(context.Context, control.Request) (control.Response, error) {
		return control.Response{OK: true}, nil
	}, func(context.Context) any { return nil }, channel)
	get := func(token string) (int, map[string]any) {
		r := httptest.NewRequest("GET", "http://127.0.0.1:1234/api/update-progress", nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	if code, _ := get(""); code != 401 {
		t.Fatalf("no token: %d", code)
	}
	if code, out := get("secret"); code != 200 || out["downloading"] != false {
		t.Fatalf("idle: %d %v", code, out)
	}
	if err := os.WriteFile(filepath.Join(channel.Directory, "download.progress"), []byte(`{"version":"0.0.0-preview.74","done":300,"total":1200}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out := get("secret"); code != 200 || out["downloading"] != true || out["done"] != float64(300) || out["total"] != float64(1200) {
		t.Fatalf("downloading: %d %v", code, out)
	}
}
