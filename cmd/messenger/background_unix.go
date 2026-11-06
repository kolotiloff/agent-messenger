//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/supervisor"
)

type backgroundReady struct {
	PID   int    `json:"pid"`
	Error string `json:"error,omitempty"`
}

// socketpair is private to this launch. A pre-existing control socket cannot
// impersonate readiness, and the parent cannot mistake another daemon for ours.
func startupPair() (net.Conn, *os.File, error) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, err
	}
	syscall.CloseOnExec(fds[0])
	syscall.CloseOnExec(fds[1])
	parent := os.NewFile(uintptr(fds[0]), "startup-parent")
	child := os.NewFile(uintptr(fds[1]), "startup-child")
	conn, err := net.FileConn(parent)
	parent.Close()
	if err != nil {
		child.Close()
		return nil, nil, err
	}
	return conn, child, nil
}

func startBackground(root string, flags []string) (int, string, error) {
	executable, err := os.Executable()
	if err != nil {
		return 0, "", err
	}
	return startBackgroundAt(root, flags, executable)
}

func startBackgroundAt(root string, flags []string, executable string) (int, string, error) {
	// Validate here too: callers other than the CLI must not smuggle child modes.
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
	if err := os.Chmod(root, 0o700); err != nil {
		return 0, "", err
	}
	conn, child, err := startupPair()
	if err != nil {
		return 0, "", err
	}
	defer conn.Close()
	defer child.Close()
	// Unique 0600 file; never truncate an old log or follow an existing symlink.
	log, err := os.CreateTemp(root, "background-*.log")
	if err != nil {
		return 0, "", err
	}
	defer log.Close()
	args := append([]string{"--root", root, "_background-serve"}, flags...)
	cmd := exec.Command(executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout, cmd.Stderr = log, log
	cmd.ExtraFiles = []*os.File{child} // child fd 3
	// nil stdin is /dev/null; the parent terminal and its pipes are not retained.
	if err := cmd.Start(); err != nil {
		return 0, log.Name(), err
	}
	child.Close()
	committed := false
	defer func() {
		if !committed {
			// No dispatch loop before handshake, and detached startup is paused.
			// This is our unreaped child, never a PID loaded from an old file.
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	if err := conn.SetDeadline(time.Now().Add(12 * time.Second)); err != nil {
		return 0, log.Name(), err
	}
	var ready backgroundReady
	if err := json.NewDecoder(io.LimitReader(conn, 65536)).Decode(&ready); err != nil {
		return 0, log.Name(), fmt.Errorf("фоновый запуск не подтверждён; журнал %s: %w", log.Name(), err)
	}
	if ready.Error != "" {
		return 0, log.Name(), errors.New(ready.Error)
	}
	if ready.PID != cmd.Process.Pid {
		return 0, log.Name(), errors.New("неверное подтверждение фонового процесса")
	}
	if err := json.NewEncoder(conn).Encode(true); err != nil {
		return 0, log.Name(), err
	}
	committed = true
	// The short-lived command normally exits now; keep tests/embedded callers
	// from leaving a zombie while they stay alive. Never kill on parent exit.
	go func() { _ = cmd.Wait() }()
	return ready.PID, log.Name(), nil
}

func runBackgroundChild(root string, flags []string) error {
	options, err := serveOptions(flags)
	if err != nil {
		return err
	}
	file := os.NewFile(3, "startup-child")
	if file == nil {
		return errors.New("внутренний запуск требует родительский канал; используйте start")
	}
	conn, err := net.FileConn(file)
	file.Close()
	if err != nil {
		return fmt.Errorf("внутренний запуск требует родительский канал; используйте start: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	ready := func() error {
		if err := json.NewEncoder(conn).Encode(backgroundReady{PID: os.Getpid()}); err != nil {
			return err
		}
		var commit bool
		if err := json.NewDecoder(io.LimitReader(conn, 64)).Decode(&commit); err != nil {
			return err
		}
		if !commit {
			return errors.New("фоновый запуск отменён до начала обработки очереди")
		}
		return conn.Close()
	}
	err = supervisor.RunWithOptions(context.Background(), root,
		options,
		supervisor.RunOptions{StartPaused: true, Ready: ready})
	if err != nil {
		_ = json.NewEncoder(conn).Encode(backgroundReady{Error: err.Error()})
	}
	return err
}
