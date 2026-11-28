//go:build !darwin && !linux && !windows

package platform

import (
	"context"
	"errors"
)

func HoldAwake(context.Context) error {
	return errors.New("system sleep control unavailable")
}
