package core

import "errors"

var (
	ErrRejected        = errors.New("rejected")
	ErrBusy            = errors.New("busy")
	ErrPaused          = errors.New("paused")
	ErrEmpty           = errors.New("empty")
	ErrClosed          = errors.New("closed")
	ErrRecoveryBlocked = errors.New("recovery blocked")
	ErrNotVerified     = errors.New("result not verified")
	ErrBadConfirmation = errors.New("retry confirmation missing")
)

// RetryConfirmation is the explicit operator phrase required before a new attempt.
const RetryConfirmation = "actions-may-repeat"
