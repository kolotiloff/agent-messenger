package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/platform"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

// runUpdateFind only looks for a newer signed release and downloads no
// package: the Mac shows the update icon from it, and downloads when the
// owner presses it, as on Windows and Linux (T-213).
func runUpdateFind(root string, args []string) error {
	if root != "" || len(args) != 0 {
		return errors.New("update-find takes no arguments or client root")
	}
	return runUpdateChannel(false)
}

// This command cannot reach supervisor/control or provider launch code.
func runUpdateFetch(root string, args []string) error {
	if root != "" || len(args) != 0 {
		return errors.New("update-fetch takes no arguments or client root")
	}
	return runUpdateChannel(true)
}

func runUpdateChannel(fetch bool) error {
	home, err := platform.DefaultRoot()
	if err != nil {
		return err
	}
	version := buildinfo.Current().Version
	index := strings.LastIndex(version, ".")
	build := 0
	if index >= 0 {
		build, _ = strconv.Atoi(version[index+1:])
	}
	// Each transfer has its own limits: a stalled one ends after a minute
	// without data, a slow one may go on (T-203).
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	channel := updatefeed.Channel{Directory: filepath.Join(home, "updates"), PublicKey: buildinfo.UpdatePublicKey}
	var result updatefeed.SyncResult
	if fetch {
		result, err = channel.SyncGitHub(ctx, build)
	} else {
		result, err = channel.FindGitHub(ctx, build)
	}
	if errors.Is(err, updatefeed.ErrDownloadBusy) {
		return json.NewEncoder(os.Stdout).Encode(updatefeed.SyncResult{State: "busy"})
	}
	// GitHub's hourly limit is not a broken update: the app says when to
	// try again instead of a download failure.
	var limit updatefeed.RateLimitError
	if errors.As(err, &limit) {
		out := updatefeed.SyncResult{State: "rate_limited"}
		if !limit.Reset.IsZero() {
			out.RetryAt = limit.Reset.UTC().Format(time.RFC3339)
		}
		return json.NewEncoder(os.Stdout).Encode(out)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
