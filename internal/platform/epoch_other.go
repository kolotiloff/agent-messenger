//go:build !darwin

package platform

import "fmt"

func CurrentRecoveryEpoch() (RecoveryEpoch, error) {
	return RecoveryEpoch{}, fmt.Errorf("%w: stream recovery epoch is currently implemented only on macOS", ErrUnsupported)
}
