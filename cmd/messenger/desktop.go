package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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
	if err := ensureDesktopClient(ctx, root); err != nil {
		return err
	}
	home, err := platform.DefaultRoot()
	if err != nil {
		return err
	}
	channel := &updatefeed.Channel{Directory: filepath.Join(home, "updates"), PublicKey: buildinfo.UpdatePublicKey}
	return localui.RunDesktop(ctx, root, 0, os.Stdout, channel, desktopRestart(root))
}

func desktopRestart(root string) localui.Restarter {
	return func(ctx context.Context) error {
		return restartClient(ctx, root, func() error { return startDesktopClient(root) }, time.Second)
	}
}

// ensureDesktopClient starts the background client if none answers, as the
// window always did.
func ensureDesktopClient(ctx context.Context, root string) error {
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
		if err := startDesktopClient(root); err != nil {
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
	return nil
}

// startDesktopClient starts the background client the way the window does.
// No --allow-model-calls can reach this launch. Task workers are allowed
// (D-029): the owner starts them from the coordinator chat and stops
// everything with the pause. The client always starts paused.
func startDesktopClient(root string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return startClientFrom(root, executable)
}

// startClientFrom starts the background client from a runtime copy of the
// program file executable.
func startClientFrom(root, executable string) error {
	home, err := platform.DefaultRoot()
	if err != nil {
		return err
	}
	executable, err = distribution.CacheRuntime(executable, filepath.Join(home, "runtimes"))
	if err != nil {
		return err
	}
	_, _, err = startBackgroundAt(root, []string{"--claude-stream", "--allow-tasks"}, executable)
	return err
}

// restartClient replaces an older background client with this build's one,
// from the window's button. It uses the client's own shutdown command, the
// same one the owner used to type: streams are stopped and recorded, task
// workers keep running and their runs stay reserved until the owner
// decides. The new client starts only after the old one stopped answering;
// the client lock still decides whether it may run. A pause set by the
// owner is kept.
func restartClient(ctx context.Context, root string, start func() error, step time.Duration) error {
	call := func(cmd string, limit time.Duration) (control.Response, error) {
		c, cancel := context.WithTimeout(ctx, limit)
		defer cancel()
		return control.CallContext(c, root, control.Request{Cmd: cmd})
	}
	response, err := call("status", 3*time.Second)
	if err != nil || !response.OK || response.Status == nil {
		return errors.New("фоновый клиент не ответил. Ничего не остановлено")
	}
	paused := response.Status.Paused
	response, err = call("shutdown", 5*time.Second)
	if err == nil && !response.OK {
		return errors.New("фоновый клиент отказался остановиться. Новый не запущен")
	}
	// Without an answer the shutdown may still have happened; the client
	// is treated as gone only when it stops answering.
	gone := false
	for i := 0; i < 15 && ctx.Err() == nil; i++ {
		if _, err := call("status", step); err != nil {
			gone = true
			break
		}
		sleep(ctx, step/4)
	}
	if !gone {
		return errors.New("старый клиент ещё отвечает. Новый не запущен; проверьте состояние и повторите")
	}
	// The old process may still hold the client lock for a moment while it
	// exits. A start refused by the lock leaves nothing behind.
	for i := 0; ; i++ {
		err = start()
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), platform.ErrAlreadyRunning.Error()) || i >= 20 || ctx.Err() != nil {
			// The detail may name local paths; the window shows only the outcome.
			return errors.New("старый клиент остановлен, а новый не запустился. Закройте и снова откройте Messenger: окно запустит клиент само")
		}
		sleep(ctx, step/2)
	}
	if !paused {
		// The new client starts paused, as at the window's first launch.
		if response, err := call("resume", 3*time.Second); err != nil || !response.OK {
			return errors.New("новый клиент запущен, но остался на паузе. Нажмите «Продолжить»")
		}
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
