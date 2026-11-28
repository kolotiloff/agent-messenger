package platform

import (
	"context"
	"errors"
	"runtime"

	"golang.org/x/sys/windows"
)

var setThreadExecutionState = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// HoldAwake owns its execution state on one OS thread until cancellation.
func HoldAwake(ctx context.Context) error {
	const continuous, systemRequired = 0x80000000, 0x00000001
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if value, _, _ := setThreadExecutionState.Call(continuous | systemRequired); value == 0 {
		return errors.New("could not prevent system sleep")
	}
	defer setThreadExecutionState.Call(continuous)
	<-ctx.Done()
	return ctx.Err()
}
