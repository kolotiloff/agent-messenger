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
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Background watch for the coordinator (T-120). A chat that blocks in wait
// does not see the owner's messages until its turn ends, so the coordinator
// runs this command in the background with its shell tool and ends its
// turn. The command exits when an event of the project's tasks arrives after
// the cursor; the chat is then woken with its output and reads the events
// with wait. The watch leaves the cursor and chat ownership alone.

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
	reader, err := core.OpenTaskEventReader(dbPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	read := func(ctx context.Context, project, audience string, after int64, limit int) ([]core.TaskEvent, int64, error) {
		events, cursor, err := reader.TaskEvents(ctx, project, audience, after, limit)
		if err != nil || len(events) > 0 {
			return events, cursor, err
		}
		pending, err := reader.PendingProjectNotice(ctx, project)
		if err != nil {
			return nil, after, err
		}
		if pending {
			return nil, after, errWatchProjectNotice
		}
		overdue, err := reader.MacUpdateRestartOverdue(ctx, time.Now())
		if overdue {
			return nil, after, errWatchUpdateRestartOverdue
		}
		return nil, after, err
	}
	return watchCoordinator(ctx, read, *project, *after, out)
}

func waitingEvents(store *core.Store, project string, after int64) ([]core.TaskEvent, error) {
	if err := store.CheckExpiredMacUpdateRestart(time.Now()); err != nil {
		return nil, err
	}
	events, _, err := store.TaskEvents(project, core.AudienceCoordinator, after, coordinatorMaxReturned+1)
	return events, err
}

var errWatchUpdateRestartOverdue = errors.New("planned Messenger restart deadline expired")
var errWatchProjectNotice = errors.New("a project notice is ready")

func watchCoordinator(ctx context.Context, read func(context.Context, string, string, int64, int) ([]core.TaskEvent, int64, error), project string, after int64, out io.Writer) error {
	tick := time.NewTicker(coordinatorWatchPoll)
	defer tick.Stop()
	checked := false
	for ctx.Err() == nil {
		events, _, err := read(ctx, project, core.AudienceCoordinator, after, coordinatorMaxReturned+1)
		switch {
		case errors.Is(err, errWatchProjectNotice):
			_, err := fmt.Fprintf(out, "Messenger: a notice is ready for project %s. Call wait with after=%d and timeout_seconds 1 to read it.\n", project, after)
			return err
		case errors.Is(err, errWatchUpdateRestartOverdue):
			_, err := fmt.Fprintf(out, "Messenger: the planned restart deadline passed. Call wait for project %s with after=%d and timeout_seconds 1 to check network contact. Keep the original cursor until wait returns a new one.\n", project, after)
			return err
		case err == nil:
			checked = true
		case sqliteBusy(err):
			checked = false
		default:
			if !errors.Is(err, ctx.Err()) {
				return fmt.Errorf("read coordinator events for project %q: %w", project, err)
			}
		}
		if err == nil && len(events) > 0 {
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
			_, err := fmt.Fprintf(out, "Messenger: %s new event(s) of project %s after cursor %d (%s). Call wait with after=%d and timeout_seconds 1 to read them, act on them, then start this watch again with the new cursor.\n",
				count, project, after, strings.Join(names, ", "), after)
			return err
		}
		select {
		case <-ctx.Done():
		case <-tick.C:
		}
	}
	if !checked {
		_, err := fmt.Fprintf(out, "Messenger: could not check new events of project %s after cursor %d before the watch ended. Start this watch again with the same cursor.\n", project, after)
		return err
	}
	_, err := fmt.Fprintf(out, "Messenger: no new events of project %s after cursor %d. Start this watch again with the same cursor to keep watching.\n", project, after)
	return err
}

func sqliteBusy(err error) bool {
	var dbErr *sqlite.Error
	// Extended busy codes have SQLITE_BUSY in their low byte.
	return errors.As(err, &dbErr) && dbErr.Code()&0xff == sqlite3.SQLITE_BUSY
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
