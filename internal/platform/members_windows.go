//go:build windows

package platform

import (
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// jobProcessList is JOBOBJECT_BASIC_PROCESS_ID_LIST with room for 64 IDs.
type jobProcessList struct {
	Assigned uint32
	Listed   uint32
	IDs      [64]uintptr
}

// GroupMembers lists the processes of a run's job with their names. Only a
// job this client holds or can still open by name can be listed.
func GroupMembers(pgid int) ([]Member, error) {
	if pgid <= 0 {
		return nil, fmt.Errorf("%w: process group was not recorded", ErrNotProven)
	}
	jobs.Lock()
	job, held := jobs.open[pgid]
	jobs.Unlock()
	if !held {
		opened, err := openJob(pgid, jobQuery)
		if err != nil {
			return nil, fmt.Errorf("%w: job cannot be opened: %v", ErrNotProven, err)
		}
		defer windows.CloseHandle(opened)
		job = opened
	}
	var list jobProcessList
	if err := windows.QueryInformationJobObject(job, windows.JobObjectBasicProcessIdList, uintptr(unsafe.Pointer(&list)), uint32(unsafe.Sizeof(list)), nil); err != nil && err != windows.ERROR_MORE_DATA {
		return nil, fmt.Errorf("query job processes: %v", err)
	}
	n := int(list.Listed)
	if n > len(list.IDs) {
		n = len(list.IDs)
	}
	out := make([]Member, 0, n)
	for _, id := range list.IDs[:n] {
		out = append(out, Member{PID: int(id), Name: processName(uint32(id))})
	}
	return out, nil
}

func processName(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	return filepath.Base(windows.UTF16ToString(buf[:size]))
}
