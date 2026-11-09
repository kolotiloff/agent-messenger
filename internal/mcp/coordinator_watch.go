package mcp

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/core"
)

// Background watch for the coordinator (T-120). A chat that blocks in wait
// does not see the owner's messages until its turn ends, so the coordinator
// runs this command in the background with its shell tool and ends its
// turn. The command exits when an event of the project's tasks arrives after
// the cursor; the chat is then woken with its output and reads the events
// with wait. It only looks: it never moves the cursor or claims the chat.

// CoordinatorWatchCommand is the subcommand of the background watch.
const CoordinatorWatchCommand = "coordinator-watch"

// CoordinatorProgramVariable passes the program file the relay runs to the
// coordinator server, so the watch command it gives names that file.
const CoordinatorProgramVariable = "AGENT_MESSENGER_COORDINATOR_PROGRAM"

// coordinatorWatchLimit is how long one watch runs at most.
const coordinatorWatchLimit = 30 * time.Minute

// coordinatorWatchPoll is how often the watch looks; a variable for tests.
var coordinatorWatchPoll = time.Second

// CoordinatorWatchMain runs `coordinator-watch --project NAME --after N`.
func CoordinatorWatchMain(dbPath string, args []string, out io.Writer) error {
	fs := flag.NewFlagSet(CoordinatorWatchCommand, flag.ContinueOnError)
	project := fs.String("project", "", "project name")
	after := fs.Int64("after", -1, "cursor from the last wait")
	minutes := fs.Int("minutes", int(coordinatorWatchLimit/time.Minute), "how long to watch at most")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !core.ValidProjectName(*project) {
		return errors.New("coordinator-watch requires --project NAME")
	}
	if *after < 0 {
		return errors.New("coordinator-watch requires --after CURSOR (the cursor of your last wait or tasks)")
	}
	limit := time.Duration(*minutes) * time.Minute
	if limit <= 0 || limit > coordinatorWatchLimit {
		limit = coordinatorWatchLimit
	}
	store, err := core.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	return watchCoordinator(ctx, store, *project, *after, out)
}

func watchCoordinator(ctx context.Context, store *core.Store, project string, after int64, out io.Writer) error {
	tick := time.NewTicker(coordinatorWatchPoll)
	defer tick.Stop()
	touched := time.Time{}
	for {
		events, _, err := store.TaskEvents(project, core.AudienceCoordinator, after, coordinatorMaxReturned+1)
		if err != nil {
			return err
		}
		if len(events) > 0 {
			count := fmt.Sprint(len(events))
			if len(events) > coordinatorMaxReturned {
				count = fmt.Sprintf("more than %d", coordinatorMaxReturned)
			}
			kinds := map[string]bool{}
			var names []string
			for _, e := range events {
				if !kinds[e.Kind] {
					kinds[e.Kind] = true
					names = append(names, e.Kind)
				}
			}
			fmt.Fprintf(out, "Messenger: %s new event(s) of project %s after cursor %d (%s). Call wait with after=%d and timeout_seconds 1 to read them, act on them, then start this watch again with the new cursor.\n",
				count, project, after, strings.Join(names, ", "), after)
			return nil
		}
		// The chat is alive while it watches, so the window shows it as
		// connected.
		if time.Since(touched) > time.Minute {
			_ = store.TouchCoordinator(project)
			touched = time.Now()
		}
		select {
		case <-ctx.Done():
			fmt.Fprintf(out, "Messenger: no new events of project %s after cursor %d. Start this watch again with the same cursor to keep watching.\n", project, after)
			return nil
		case <-tick.C:
		}
	}
}

// coordinatorWatchLine is the command the coordinator runs in the
// background, for the shell of its chat (POSIX quoting; Claude Code uses
// Git Bash on Windows).
func coordinatorWatchLine(dbPath, project string) string {
	program := os.Getenv(CoordinatorProgramVariable)
	if program == "" {
		program, _ = os.Executable()
	}
	root := strings.TrimSuffix(dbPath, "queue.sqlite")
	root = strings.TrimRight(root, `/\`)
	return shellWord(program) + " --root " + shellWord(root) + " " + CoordinatorWatchCommand + " --project " + shellWord(project) + " --after CURSOR"
}

func shellWord(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
