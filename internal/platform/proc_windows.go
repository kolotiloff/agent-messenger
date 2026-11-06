//go:build windows

package platform

import "fmt"

func Snapshot() (string, []Proc, error) {
	return "", nil, fmt.Errorf("%w: process snapshot has no tested Windows implementation", ErrUnsupported)
}

func ProcessTree() (string, []Proc, error) {
	return "", nil, fmt.Errorf("%w: process tree has no tested Windows implementation", ErrUnsupported)
}

func ReadObservation(pid int) (Observation, error) {
	return Observation{}, fmt.Errorf("%w: process observation has no tested Windows implementation", ErrUnsupported)
}

func GroupEmpty(pgid int) (bool, error) {
	return false, fmt.Errorf("%w: process-group check has no tested Windows implementation", ErrUnsupported)
}

func SignalGroup(pgid int) error {
	return fmt.Errorf("%w: process-group signal has no tested Windows implementation", ErrUnsupported)
}
