package main

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
)

// staleWindowProcess returns the process of the window that answered on
// conn when it runs an older installed version of this program (T-209).
func staleWindowProcess(conn net.Conn) (int, bool) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, false
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return 0, false
	}
	var cred *unix.Ucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil || credErr != nil || cred == nil {
		return 0, false
	}
	if cred.Pid <= 1 || int(cred.Uid) != os.Getuid() || int(cred.Pid) == os.Getpid() {
		return 0, false
	}
	other, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(int(cred.Pid)), "exe"))
	if err != nil {
		return 0, false
	}
	own, err := os.Executable()
	if err == nil {
		own, err = filepath.EvalSymlinks(own)
	}
	if err != nil {
		return 0, false
	}
	return int(cred.Pid), olderWindowProgram(other, own, buildinfo.Version)
}

// closeStaleWindow stops an older window and reports whether its socket
// stopped answering. The window holds no task: the client is its own process.
func closeStaleWindow(pid int, socket string) bool {
	if syscall.Kill(pid, syscall.SIGTERM) != nil {
		return false
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", socket, 200*time.Millisecond)
		if err != nil {
			return true
		}
		_ = conn.Close()
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
