package platform

import (
	"fmt"
	"regexp"
)

// RecoveryEpoch is independent of legacy wall-clock boot timestamps. Old
// observations must never become proof merely because their format changed.
type RecoveryEpoch struct {
	Version int    `json:"version"`
	Host    string `json:"host"`
	Boot    string `json:"boot"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var hostPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (e RecoveryEpoch) Valid() bool {
	return e.Version == 1 && hostPattern.MatchString(e.Host) && uuidPattern.MatchString(e.Boot) && e.Boot != "00000000-0000-0000-0000-000000000000"
}

func ProvePreviousBoot(stored, current RecoveryEpoch) (Proof, error) {
	if !stored.Valid() || !current.Valid() {
		return Proof{}, fmt.Errorf("%w: no versioned recovery epoch; legacy records cannot be upgraded retroactively", ErrNotProven)
	}
	if stored.Host != current.Host {
		return Proof{}, fmt.Errorf("%w: reservation belongs to another host", ErrNotProven)
	}
	if stored.Boot == current.Boot {
		return Proof{}, fmt.Errorf("%w: same boot; absence of a PID or group is not proof", ErrNotProven)
	}
	return Proof{Proven: true, Detail: "same host, different boot session UUID; previous boot processes are gone"}, nil
}
