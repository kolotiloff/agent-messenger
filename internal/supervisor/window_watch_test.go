package supervisor

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

func TestWindowWatchThroughControlWakesOnCommittedChange(t *testing.T) {
	f := newLinkFixture(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	f.sup.ctx = ctx
	listener, err := control.Listen(f.sup.root)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go control.Serve(listener, f.sup)

	watch := func(window, feed string) (core.WindowChanges, error) {
		reply, err := control.CallContext(ctx, f.sup.root, control.Request{
			Cmd: "window-changes", Alias: f.project, Before: window, Confirmation: feed,
		})
		var change core.WindowChanges
		if err == nil {
			err = json.Unmarshal(reply.Overview, &change)
		}
		return change, err
	}
	base, err := watch("", "")
	if err != nil || base.Window == "" {
		t.Fatalf("initial revision: %+v %v", base, err)
	}
	changes := make(chan core.WindowChanges, 1)
	errs := make(chan error, 1)
	go func() {
		change, err := watch(base.Window, base.Feed)
		changes <- change
		errs <- err
	}()
	select {
	case change := <-changes:
		t.Fatalf("unchanged subscription returned immediately: %+v %v", change, <-errs)
	case <-time.After(2 * windowWatchInterval):
	}
	if err := f.sup.store.SetAgentPaused("claude", true); err != nil {
		t.Fatal(err)
	}
	select {
	case change := <-changes:
		if err := <-errs; err != nil || change.Window == base.Window {
			t.Fatalf("change: %+v %v", change, err)
		}
	case <-ctx.Done():
		t.Fatal("subscription did not wake after the commit")
	}
}
