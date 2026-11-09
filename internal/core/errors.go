package core

import "errors"

var (
	ErrRejected        = errors.New("rejected")
	ErrBusy            = errors.New("busy")
	ErrPaused          = errors.New("paused")
	ErrClosed          = errors.New("closed")
	ErrRecoveryBlocked = errors.New("recovery blocked")
	ErrBadConfirmation = errors.New("recovery confirmation missing")
)

// RecoveryConfirmation is the explicit operator phrase required before a
// proven-reboot recovery retires a reservation (recover-missing-root).
const RecoveryConfirmation = "preserve-uncertain"
