//go:build windows

package platform

import "fmt"

// Lock is unused on Windows until a tested lock implementation exists.
type Lock struct{}

func Acquire(path string) (*Lock, error) {
	return nil, fmt.Errorf("%w: single-instance lock has no tested Windows implementation", ErrUnsupported)
}

func (l *Lock) Release() error { return nil }
