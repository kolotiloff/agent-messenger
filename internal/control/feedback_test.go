package control

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

type feedbackHandler struct {
	statusHandler
	project, text, kind string
	image               []byte
}

func (h *feedbackHandler) AddFeedback(project, text, kind string, image []byte) error {
	h.project, h.text, h.kind, h.image = project, text, kind, image
	return nil
}

func (h *feedbackHandler) ExportFeedback() (string, int, error) { return "/x/feedback.zip", 3, nil }

// T-135: a remark from the window reaches the client with its picture, and
// an export answers with the file's path.
func TestFeedbackCommands(t *testing.T) {
	h := &feedbackHandler{}
	r := dispatch(h, Request{Cmd: "feedback-add", Alias: "Game", Text: "slow", Action: "image/jpeg", Data: base64.StdEncoding.EncodeToString([]byte("jpg"))})
	if !r.OK || h.project != "Game" || h.text != "slow" || h.kind != "image/jpeg" || string(h.image) != "jpg" {
		t.Fatalf("add %+v %+v", r, h)
	}
	if r := dispatch(h, Request{Cmd: "feedback-add", Text: "x", Data: "%%%"}); r.OK {
		t.Fatal("a damaged picture was accepted")
	}
	r = dispatch(h, Request{Cmd: "feedback-export"})
	var out struct {
		Path    string `json:"path"`
		Remarks int    `json:"remarks"`
	}
	if !r.OK || json.Unmarshal(r.Overview, &out) != nil || out.Path != "/x/feedback.zip" || out.Remarks != 3 {
		t.Fatalf("export %+v", r)
	}
}
