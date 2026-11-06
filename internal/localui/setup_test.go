package localui

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
)

func TestSetupOnlyForwardsFixedNewSessionCommand(t *testing.T) {
	const body = `{"id":"a78f1366-38e7-4cec-b0b2-98670cfd79b6","alias":"test","cli":"/trusted/claude","environment":"/profiles/test"}`
	var got []control.Request
	h := newHandler(actionHost, "secret", func(_ context.Context, r control.Request) (control.Response, error) {
		got = append(got, r)
		return control.Response{OK: true}, nil
	}, nil)
	for _, bad := range []string{`null`, body + ` {}`, strings.Replace(body, "a78f1366-38e7-4cec-b0b2-98670cfd79b6", "../escape", 1), strings.TrimSuffix(body, `}`) + `,"workspace":"/working-project"}`, strings.TrimSuffix(body, `}`) + `,"cmd":"enqueue"}`} {
		if w := postAction(h, "setup-claude", bad); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if len(got) != 0 {
		t.Fatal("invalid setup reached controller")
	}
	w := postAction(h, "setup-claude", body)
	if w.Code != http.StatusOK || len(got) != 1 || got[0].Cmd != "setup-new-claude" || got[0].Workspace != "" || got[0].Text != "" || got[0].ConversationID != "a78f1366-38e7-4cec-b0b2-98670cfd79b6" {
		t.Fatal(w.Code, w.Body.String(), got)
	}
}
