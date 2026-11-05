//go:build darwin

package platform

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var platformUUIDPattern = regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([0-9A-Fa-f-]+)"`)

func CurrentRecoveryEpoch() (RecoveryEpoch, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	boot, err := exec.CommandContext(ctx, "/usr/sbin/sysctl", "-n", "kern.bootsessionuuid").Output()
	if err != nil {
		return RecoveryEpoch{}, fmt.Errorf("%w: boot session UUID unavailable", ErrNotProven)
	}
	host, err := exec.CommandContext(ctx, "/usr/sbin/ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return RecoveryEpoch{}, fmt.Errorf("%w: host identity unavailable", ErrNotProven)
	}
	matches := platformUUIDPattern.FindAllSubmatch(host, -1)
	if len(matches) != 1 {
		return RecoveryEpoch{}, fmt.Errorf("%w: host identity ambiguous", ErrNotProven)
	}
	id := strings.ToLower(string(matches[0][1]))
	if !uuidPattern.MatchString(id) || id == "00000000-0000-0000-0000-000000000000" {
		return RecoveryEpoch{}, ErrNotProven
	}
	hash := sha256.Sum256([]byte("agent-messenger/macos-host/v1:" + id))
	epoch := RecoveryEpoch{Version: 1, Host: fmt.Sprintf("%x", hash), Boot: strings.ToLower(strings.TrimSpace(string(boot)))}
	if !epoch.Valid() {
		return RecoveryEpoch{}, ErrNotProven
	}
	return epoch, nil
}
