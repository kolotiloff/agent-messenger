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

// This command cannot reach supervisor/control or provider launch code.
func runUpdateFetch(root string, args []string) error {
	if root != "" || len(args) != 0 {
		return errors.New("update-fetch takes no arguments or client root")
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	channel := updatefeed.Channel{Directory: filepath.Join(home, "updates"), PublicKey: buildinfo.UpdatePublicKey}
	result, err := channel.SyncGitHub(ctx, build)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
