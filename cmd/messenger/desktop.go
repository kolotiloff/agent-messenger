package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/distribution"
	"github.com/kolotiloff/agent-messenger/internal/localui"
	"github.com/kolotiloff/agent-messenger/internal/platform"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

// desktop is an app-owned UI child. EOF on stdin ends only this HTTP server;
// the independently owned client and all session reservations remain intact.
func runDesktop(root string, args []string) error {
	if len(args) != 0 {
		return errors.New("desktop takes no model permissions or extra arguments")
	}
	if root == "" {
		var err error
		root, err = supervisor.DesktopRoot()
		if err != nil {
			return fmt.Errorf("не удалось выбрать сохранённые данные: %w", err)
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	go func() { _, _ = io.Copy(io.Discard, os.Stdin); cancel() }()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	statusCtx, stop := context.WithTimeout(ctx, 2*time.Second)
	response, err := control.CallContext(statusCtx, root, control.Request{Cmd: "status"})
	stop()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil || !response.OK || response.Status == nil {
		// The global supervisor/startup locks arbitrate even if status timed out.
		// No --allow-model-calls can reach this launch. Task workers are
		// allowed (D-029): the owner starts them from the coordinator chat and
		// stops everything with the pause. start always begins paused.
		executable, cacheErr := os.Executable()
		if cacheErr != nil {
			return cacheErr
		}
		home, cacheErr := platform.DefaultRoot()
		if cacheErr != nil {
			return cacheErr
		}
		executable, cacheErr = distribution.CacheRuntime(executable, filepath.Join(home, "runtimes"))
		if cacheErr != nil {
			return cacheErr
		}
		if _, _, err = startBackgroundAt(root, []string{"--claude-stream", "--allow-tasks"}, executable); err != nil {
			return fmt.Errorf("клиент недоступен; другой экземпляр или защитный резерв могут удерживать его. Выберите прежний каталог данных: %w", err)
		}
		// D-029: a client the window started itself runs tasks at once. This
		// launches no model for queued session messages (the window has not
		// authorized any session yet) and never repeats an uncertain run. If
		// it fails, the window shows the pause with its "Продолжить" button.
		resumeCtx, stopResume := context.WithTimeout(ctx, 3*time.Second)
		_, _ = control.CallContext(resumeCtx, root, control.Request{Cmd: "resume"})
		stopResume()
	}
	home, err := platform.DefaultRoot()
	if err != nil {
		return err
	}
	channel := &updatefeed.Channel{Directory: filepath.Join(home, "updates"), PublicKey: buildinfo.UpdatePublicKey}
	return localui.RunWithUpdates(ctx, root, 0, os.Stdout, channel)
}
