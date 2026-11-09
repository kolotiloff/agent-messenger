package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/selfupdate"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

// The helper's decisions without a release: tasks hold it back, a failed
// check and a download owned by the window are failures, an older client
// is restarted and an up-to-date one is left alone. Installing a signed
// update is the selfupdate package's tested Apply.
func TestRemoteUpdateDecisions(t *testing.T) {
	type outcome struct{ state, version, text string }
	setup := func(syncErr error, work int, client string) (*remoteUpdate, *[]string) {
		var restarts []string
		u := &remoteUpdate{
			program: "/opt/messenger",
			running: "0.0.0-preview.57",
			updater: &selfupdate.Updater{
				Channel:    updatefeed.Channel{Directory: t.TempDir(), PublicKey: "a2V5"},
				Executable: "/opt/messenger",
				Running:    57,
				Sync:       func(context.Context) error { return syncErr },
			},
			activeWork:    func() (int, error) { return work, nil },
			clientVersion: func(context.Context) (string, error) { return client, nil },
			restart: func(_ context.Context, program string) error {
				restarts = append(restarts, program)
				return nil
			},
			record: func(string) error { return nil },
		}
		return u, &restarts
	}
	run := func(u *remoteUpdate) outcome {
		s, v, x := u.run(context.Background())
		return outcome{s, v, x}
	}

	u, restarts := setup(nil, 2, "0.0.0-preview.57")
	if got := run(u); got.state != core.UpdateRefused || !strings.Contains(got.text, "(2)") || len(*restarts) != 0 {
		t.Fatalf("tasks %+v %v", got, *restarts)
	}
	u, restarts = setup(nil, 0, "0.0.0-preview.57")
	if got := run(u); got.state != core.UpdateDone || got.version != "0.0.0-preview.57" || got.text != "уже последняя версия" || len(*restarts) != 0 {
		t.Fatalf("latest %+v %v", got, *restarts)
	}
	u, restarts = setup(nil, 0, "0.0.0-preview.55")
	if got := run(u); got.state != core.UpdateDone || len(*restarts) != 1 || (*restarts)[0] != "/opt/messenger" {
		t.Fatalf("older client %+v %v", got, *restarts)
	}
	u, restarts = setup(errors.New("offline"), 0, "0.0.0-preview.55")
	if got := run(u); got.state != core.UpdateFailed || len(*restarts) != 0 {
		t.Fatalf("check failed %+v %v", got, *restarts)
	}
	u, restarts = setup(updatefeed.ErrDownloadBusy, 0, "0.0.0-preview.55")
	if got := run(u); got.state != core.UpdateFailed || !strings.Contains(got.text, "скачивает") || len(*restarts) != 0 {
		t.Fatalf("download busy %+v %v", got, *restarts)
	}
	u, _ = setup(nil, 0, "")
	u.updater.Channel.PublicKey = ""
	if got := run(u); got.state != core.UpdateFailed {
		t.Fatalf("unsupported %+v", got)
	}
}

// The helper's command writes its result for the client even when it
// cannot start its work.
func TestRemoteUpdateCommandArguments(t *testing.T) {
	root := t.TempDir()
	if err := runRemoteUpdate(root, []string{"--broker", "b"}); err == nil {
		t.Fatal("missing arguments accepted")
	}
	if _, err := os.Stat(filepath.Join(root, supervisor.RemoteUpdateResult)); !os.IsNotExist(err) {
		t.Fatalf("result written for a bad call: %v", err)
	}
}
