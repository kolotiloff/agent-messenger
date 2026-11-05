package control

import (
	"errors"
	"testing"
)

type overviewHandler struct {
	Handler // Other calls panic: a read must not trigger control mutations.
	value   any
	err     error
}

func (h overviewHandler) Overview() (any, error) { return h.value, h.err }

func TestOverviewDispatchReadOnlyAndErrorsAreRedacted(t *testing.T) {
	r := dispatch(overviewHandler{value: map[string]any{"schema_version": 1}}, Request{Cmd: "overview"})
	if !r.OK || string(r.Overview) != `{"schema_version":1}` || r.Status != nil || len(r.Inspect) != 0 {
		t.Fatalf("response: %+v", r)
	}
	for _, h := range []overviewHandler{{err: errors.New("private-path")}, {value: make(chan int)}} {
		r = dispatch(h, Request{Cmd: "overview"})
		if r.OK || r.Error != "overview unavailable" || len(r.Overview) != 0 {
			t.Fatalf("unsafe error: %+v", r)
		}
	}
}
