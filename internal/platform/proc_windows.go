//go:build windows

package platform

import (
	"errors"
	"fmt"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// On Windows a process's group number is the ID of the first process of its
// job (see group_windows.go). The snapshot reports every process as its own
// group: membership of other processes is checked through the job itself.

// Snapshot lists live processes with their creation time as the start token.
// A process whose times cannot be read gets no token and can never match a
// recorded one.
func Snapshot() (string, []Proc, error) {
	boot, err := bootID()
	if err != nil {
		return "", nil, err
	}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return "", nil, fmt.Errorf("%w: process snapshot: %v", ErrNotProven, err)
	}
	defer windows.CloseHandle(snap)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	var live []Proc
	for err = windows.Process32First(snap, &entry); err == nil; err = windows.Process32Next(snap, &entry) {
		pid := int(entry.ProcessID)
		if pid == 0 {
			continue
		}
		token, _ := startToken(pid)
		live = append(live, Proc{PID: pid, PGID: pid, StartToken: token})
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return "", nil, fmt.Errorf("%w: process snapshot: %v", ErrNotProven, err)
	}
	return boot, live, nil
}

func ReadObservation(pid int) (Observation, error) {
	boot, err := bootID()
	if err != nil {
		return Observation{}, err
	}
	token, err := startToken(pid)
	if err != nil {
		return Observation{}, fmt.Errorf("%w: %v", ErrNotProven, err)
	}
	return Observation{PID: pid, PGID: pid, StartToken: token, BootID: boot}, nil
}

func startToken(pid int) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return "", err
	}
	return strconv.FormatInt(created.Nanoseconds(), 10), nil
}

// bootID changes only when Windows starts again. BootId is a counter the
// system increments at every start; if it is missing, the time of the last
// orderly shutdown is used, which never changes while the system runs. Both
// are read, never derived from the clock, so a clock change cannot make a
// running boot look like a new one.
func bootID() (string, error) {
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Memory Management\PrefetchParameters`, registry.QUERY_VALUE); err == nil {
		v, _, err := k.GetIntegerValue("BootId")
		k.Close()
		if err == nil {
			return "bootid:" + strconv.FormatUint(v, 10), nil
		}
	}
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Windows`, registry.QUERY_VALUE); err == nil {
		v, _, err := k.GetBinaryValue("ShutdownTime")
		k.Close()
		if err == nil && len(v) > 0 {
			return fmt.Sprintf("shutdown:%x", v), nil
		}
	}
	return "", fmt.Errorf("%w: no boot identity in the registry", ErrNotProven)
}

// GroupEmpty reports whether the run's job has no process left. Only a job
// this client created and holds, or one it can still open by name, can be
// proven empty. A job that cannot be reached is not proven, never empty:
// Windows drops a job's name when its last handle closes even while its
// processes run.
func GroupEmpty(pgid int) (bool, error) {
	if pgid <= 0 {
		return false, fmt.Errorf("%w: process group was not recorded", ErrNotProven)
	}
	jobs.Lock()
	defer jobs.Unlock()
	if job, ok := jobs.open[pgid]; ok {
		active, err := jobActive(job)
		if err != nil {
			return false, err
		}
		if active == 0 {
			// No process can join an empty job: none is left to start one.
			windows.CloseHandle(job)
			delete(jobs.open, pgid)
			jobs.emptied[pgid] = true
			return true, nil
		}
		return false, nil
	}
	if jobs.emptied[pgid] {
		return true, nil
	}
	job, err := openJob(pgid, jobQuery)
	if err != nil {
		return false, fmt.Errorf("%w: job is not held by this client and cannot be opened: %v", ErrNotProven, err)
	}
	defer windows.CloseHandle(job)
	active, err := jobActive(job)
	if err != nil {
		return false, err
	}
	return active == 0, nil
}

func jobActive(job windows.Handle) (uint32, error) {
	var info jobAccounting
	if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)), nil); err != nil {
		return 0, fmt.Errorf("%w: query job: %v", ErrNotProven, err)
	}
	return info.ActiveProcesses, nil
}

// jobAccounting is JOBOBJECT_BASIC_ACCOUNTING_INFORMATION.
type jobAccounting struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

// SignalGroup ends every process of the run's job. Windows has no polite
// stop for a program without a console window, so this is the hard stop. A
// job this client saw empty has nothing to stop; a job it cannot reach is an
// error, because its processes may still run.
func SignalGroup(pgid int) error {
	if pgid <= 0 {
		return fmt.Errorf("%w: process group was not recorded", ErrNotProven)
	}
	jobs.Lock()
	defer jobs.Unlock()
	if job, ok := jobs.open[pgid]; ok {
		return windows.TerminateJobObject(job, 1)
	}
	if jobs.emptied[pgid] {
		return nil
	}
	job, err := openJob(pgid, jobTerminate)
	if err != nil {
		return fmt.Errorf("%w: job is not held by this client and cannot be opened: %v", ErrNotProven, err)
	}
	defer windows.CloseHandle(job)
	return windows.TerminateJobObject(job, 1)
}
