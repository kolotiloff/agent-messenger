//go:build windows

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/supervisor"
	"golang.org/x/sys/windows"
)

// readyHandleEnv names the inherited pipe handle the background child reports
// readiness on. The parent's confirmation arrives on the child's stdin. Both
// pipes are private to this launch, so another client cannot answer for it.
const readyHandleEnv = "AGENT_MESSENGER_READY_HANDLE"

type backgroundReady struct {
	PID   int    `json:"pid"`
	Error string `json:"error,omitempty"`
}

func startBackground(root string, flags []string) (int, string, error) {
	executable, err := os.Executable()
	if err != nil {
		return 0, "", err
	}
	return startBackgroundAt(root, flags, executable)
}

func startBackgroundAt(root string, flags []string, executable string) (int, string, error) {
	return startBackgroundEnv(root, flags, executable, nil)
}

// startBackgroundEnv is startBackgroundAt with extra environment entries for
// the client only.
func startBackgroundEnv(root string, flags []string, executable string, env []string) (int, string, error) {
	if _, err := serveAllowsModels(flags); err != nil {
		return 0, "", err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return 0, "", err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return 0, "", err
	}
	log, err := os.CreateTemp(root, "background-*.log")
	if err != nil {
		return 0, "", err
	}
	defer log.Close()
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		return 0, log.Name(), err
	}
	defer readyRead.Close()
	defer readyWrite.Close()
	commitRead, commitWrite, err := os.Pipe()
	if err != nil {
		return 0, log.Name(), err
	}
	defer commitRead.Close()
	defer commitWrite.Close()
	handle := windows.Handle(readyWrite.Fd())
	if err := windows.SetHandleInformation(handle, windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
		return 0, log.Name(), err
	}
	args := append([]string{"--root", root, "_background-serve"}, flags...)
	start := func(flags uint32) (*exec.Cmd, error) {
		cmd := exec.Command(executable, args...)
		cmd.Env = append(append(os.Environ(), env...), readyHandleEnv+"="+strconv.FormatUint(uint64(handle), 10))
		cmd.Stdin, cmd.Stdout, cmd.Stderr = commitRead, log, log
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags:              flags,
			HideWindow:                 true,
			AdditionalInheritedHandles: []syscall.Handle{syscall.Handle(handle)},
		}
		return cmd, cmd.Start()
	}
	// Leave the job of the terminal or window that started it, if allowed, so
	// closing that window does not end the client. Without the permission
	// the client still starts and stays a separate process group.
	base := uint32(windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW)
	cmd, err := start(base | windows.CREATE_BREAKAWAY_FROM_JOB)
	if err != nil && errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		cmd, err = start(base)
	}
	if err != nil {
		return 0, log.Name(), err
	}
	readyWrite.Close()
	commitRead.Close()
	committed := false
	defer func() {
		if !committed {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	type result struct {
		ready backgroundReady
		err   error
	}
	got := make(chan result, 1)
	go func() {
		var r backgroundReady
		err := json.NewDecoder(io.LimitReader(readyRead, 65536)).Decode(&r)
		got <- result{r, err}
	}()
	var ready backgroundReady
	select {
	case r := <-got:
		if r.err != nil {
			return 0, log.Name(), fmt.Errorf("фоновый запуск не подтверждён; журнал %s: %w", log.Name(), r.err)
		}
		ready = r.ready
	case <-time.After(12 * time.Second):
		return 0, log.Name(), fmt.Errorf("фоновый запуск не подтверждён за 12 с; журнал %s", log.Name())
	}
	if ready.Error != "" {
		return 0, log.Name(), errors.New(ready.Error)
	}
	if ready.PID != cmd.Process.Pid {
		return 0, log.Name(), errors.New("неверное подтверждение фонового процесса")
	}
	if err := json.NewEncoder(commitWrite).Encode(true); err != nil {
		return 0, log.Name(), err
	}
	commitWrite.Close()
	committed = true
	go func() { _ = cmd.Wait() }()
	return ready.PID, log.Name(), nil
}

// startDetached starts a helper outside the client's process group and,
// if allowed, its job, writing to log, and does not wait for it (T-078).
func startDetached(executable string, args []string, log *os.File) error {
	start := func(flags uint32) (*exec.Cmd, error) {
		cmd := exec.Command(executable, args...)
		cmd.Stdout, cmd.Stderr = log, log
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags, HideWindow: true}
		return cmd, cmd.Start()
	}
	base := uint32(windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW)
	cmd, err := start(base | windows.CREATE_BREAKAWAY_FROM_JOB)
	if err != nil && errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		cmd, err = start(base)
	}
	if err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func runBackgroundChild(root string, flags []string) error {
	options, err := serveOptions(flags)
	if err != nil {
		return err
	}
	raw := os.Getenv(readyHandleEnv)
	os.Unsetenv(readyHandleEnv)
	value, err := strconv.ParseUint(raw, 10, 64)
	if raw == "" || err != nil {
		return errors.New("внутренний запуск требует родительский канал; используйте start")
	}
	readyFile := os.NewFile(uintptr(value), "startup-ready")
	if readyFile == nil {
		return errors.New("внутренний запуск требует родительский канал; используйте start")
	}
	defer readyFile.Close()
	ready := func() error {
		if err := json.NewEncoder(readyFile).Encode(backgroundReady{PID: os.Getpid()}); err != nil {
			return err
		}
		type answer struct {
			commit bool
			err    error
		}
		got := make(chan answer, 1)
		go func() {
			var commit bool
			err := json.NewDecoder(io.LimitReader(os.Stdin, 64)).Decode(&commit)
			got <- answer{commit, err}
		}()
		select {
		case a := <-got:
			if a.err != nil {
				return a.err
			}
			if !a.commit {
				return errors.New("фоновый запуск отменён до начала обработки очереди")
			}
		case <-time.After(10 * time.Second):
			return errors.New("фоновый запуск не подтверждён родителем за 10 с")
		}
		return readyFile.Close()
	}
	err = supervisor.RunWithOptions(context.Background(), root,
		options,
		supervisor.RunOptions{StartPaused: true, Ready: ready, StartUpdate: remoteUpdateStarter(root)})
	if err != nil {
		_ = json.NewEncoder(readyFile).Encode(backgroundReady{Error: err.Error()})
	}
	return err
}
