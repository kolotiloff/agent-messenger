package platform

import "errors"

var (
	// ErrUnsupported means this OS build has no implementation of the operation.
	// Callers must surface it. They must not report success instead.
	ErrUnsupported = errors.New("platform operation is not supported")

	ErrAlreadyRunning = errors.New("client is already running")
	ErrNotProven      = errors.New("process stop is not proven")
	ErrStillRunning   = errors.New("process still running")
)
