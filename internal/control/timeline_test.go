package control

import (
	"errors"
	"testing"
)

type timelineHandler struct {
	Handler
	alias  string
	limit  int
	before string
	err    error
}

func (h *timelineHandler) Timeline(alias string, limit int, before string) (any, error) {
	h.alias, h.limit, h.before = alias, limit, before
	return map[string]any{"items": []string{}}, h.err
}

func TestTimelineDispatchDefaultsAndRejectsInvalidPages(t *testing.T) {
	h := &timelineHandler{}
	r := dispatch(h, Request{Cmd: "timeline", Alias: "worker", Before: "m3"})
	if !r.OK || h.alias != "worker" || h.limit != 50 || h.before != "m3" || string(r.Timeline) != `{"items":[]}` {
		t.Fatalf("response %+v", r)
	}
	for _, req := range []Request{{Cmd: "timeline"}, {Cmd: "timeline", Alias: "worker", Limit: -1}, {Cmd: "timeline", Alias: "worker", Limit: 101}} {
		h := &timelineHandler{}
		if r := dispatch(h, req); r.OK || h.alias != "" {
			t.Fatal("invalid request reached reader")
		}
	}
	h.err = errors.New("private db path and token")
	r = dispatch(h, Request{Cmd: "timeline", Alias: "worker"})
	if r.OK || r.Error != "timeline unavailable; check alias and cursor" || len(r.Timeline) != 0 {
		t.Fatalf("unredacted error %+v", r)
	}
}
