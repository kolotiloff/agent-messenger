//go:build windows

package platform

import (
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows has no process groups that a child cannot leave. A run's group is
// a named job object instead (D-043): every process the run starts belongs to
// it, and a child cannot break away because the job does not allow it. The
// job is named after the first process's ID, so a later client finds it with
// the ID stored in the database.
//
// Windows removes a job's name as soon as the last handle to it closes, even
// while its processes still run. So this client keeps the handle of every job
// it created open (see jobs) until it has seen the job empty. A job it cannot
// reach is never taken as empty: after a client restart a run's job is "not
// proven" and the task stays blocked.

const (
	jobQuery     = 0x0004 // JOB_OBJECT_QUERY
	jobTerminate = 0x0008 // JOB_OBJECT_TERMINATE
)

var (
	kernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procOpenJobObject   = kernel32.NewProc("OpenJobObjectW")
	procCreateJobObject = kernel32.NewProc("CreateJobObjectW")
)

// jobs holds the open handle of every job this client created and has not yet
// seen empty. emptied holds the jobs it saw empty; their handles are closed.
var jobs = struct {
	sync.Mutex
	open    map[int]windows.Handle
	emptied map[int]bool
}{open: map[int]windows.Handle{}, emptied: map[int]bool{}}

func jobName(pgid int) string { return fmt.Sprintf(`Local\AgentMessenger-run-%d`, pgid) }

func openJob(pgid int, access uint32) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(jobName(pgid))
	if err != nil {
		return 0, err
	}
	r, _, callErr := procOpenJobObject.Call(uintptr(access), 0, uintptr(unsafe.Pointer(name)))
	if r == 0 {
		return 0, callErr
	}
	return windows.Handle(r), nil
}

// SetNewGroup prepares the command to start suspended, without a console
// window, so StartGroup can put it in its job before it runs.
func SetNewGroup(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED | windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW
	cmd.SysProcAttr.HideWindow = true
	return nil
}

// StartGroup starts a command prepared by SetNewGroup, puts it in a new job
// named after its process ID, and only then lets it run. If any step fails,
// the still-suspended process is ended and the error wraps ErrNotStarted.
func StartGroup(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_SUSPENDED == 0 {
		return errors.New("StartGroup needs a command prepared by SetNewGroup")
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := cmd.Process.Pid
	if err := joinJob(pid); err != nil {
		// Suspended: none of its code ran. End it and wait for the exit.
		if killErr := cmd.Process.Kill(); killErr != nil {
			return fmt.Errorf("job not set up (%v); the suspended process could not be ended: %w", err, killErr)
		}
		_ = cmd.Wait()
		return fmt.Errorf("%w: %v", ErrNotStarted, err)
	}
	return nil
}

func joinJob(pid int) error {
	name, err := windows.UTF16PtrFromString(jobName(pid))
	if err != nil {
		return err
	}
	// The last error is read by the same call: it says whether the name
	// already belonged to a job.
	r, _, lastErr := procCreateJobObject.Call(0, uintptr(unsafe.Pointer(name)))
	if r == 0 {
		return fmt.Errorf("job object: %w", lastErr)
	}
	job := windows.Handle(r)
	// A job of that name still has processes of an earlier run whose first
	// process had this ID. Never share it.
	if errors.Is(lastErr, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(job)
		return errors.New("a job of an earlier run with this process ID still exists")
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("open process: %w", err)
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("assign to job: %w", err)
	}
	// Keep the handle: it keeps the job's name, so the job stays reachable
	// for GroupEmpty and SignalGroup for as long as the run lasts.
	jobs.Lock()
	if old, ok := jobs.open[pid]; ok {
		windows.CloseHandle(old)
	}
	jobs.open[pid] = job
	delete(jobs.emptied, pid)
	jobs.Unlock()
	// If resuming fails, the caller kills the process; the job then empties
	// and GroupEmpty closes the handle.
	return resumeProcess(pid)
}

// resumeProcess resumes the only thread of a process created suspended.
func resumeProcess(pid int) error {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("thread snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)
	var entry windows.ThreadEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	var threads []uint32
	for err = windows.Thread32First(snap, &entry); err == nil; err = windows.Thread32Next(snap, &entry) {
		if entry.OwnerProcessID == uint32(pid) {
			threads = append(threads, entry.ThreadID)
		}
	}
	if len(threads) != 1 {
		return fmt.Errorf("a new suspended process has %d threads, expected 1", len(threads))
	}
	thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, threads[0])
	if err != nil {
		return fmt.Errorf("open thread: %w", err)
	}
	defer windows.CloseHandle(thread)
	if _, err := windows.ResumeThread(thread); err != nil {
		return fmt.Errorf("resume: %w", err)
	}
	return nil
}

// NeverRan reports whether a failed start proves the executable never ran.
func NeverRan(cmd *exec.Cmd, err error) bool {
	if errors.Is(err, ErrNotStarted) {
		return true
	}
	return cmd.Process == nil && (errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) ||
		errors.Is(err, windows.ERROR_BAD_EXE_FORMAT) || errors.Is(err, exec.ErrNotFound))
}
