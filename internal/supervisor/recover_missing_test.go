package supervisor

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func epochOf(boot string) platform.RecoveryEpoch {
	return platform.RecoveryEpoch{Version: 1, Host: strings.Repeat("a", 64), Boot: boot}
}

func TestRecoverMissingRootsNeedsProvenReboot(t *testing.T) {
	isolateUser(t)
	old := recoveryEpoch
	t.Cleanup(func() { recoveryEpoch = old })
	existing := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone")
	if err := saveRuns([]recordedRun{
		{ID: "a", Root: missing, PID: 10, PGID: 10, StartToken: "s", BootID: "b", Epoch: epochOf("11111111-1111-4111-8111-111111111111")},
		{ID: "b", Root: existing, PID: 11, PGID: 11, StartToken: "s", BootID: "b", Epoch: epochOf("11111111-1111-4111-8111-111111111111")},
		{ID: "c", Root: filepath.Join(t.TempDir(), "gone-legacy"), PID: 12},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverMissingRoots("wrong"); !errors.Is(err, core.ErrBadConfirmation) {
		t.Fatalf("bad confirmation: %v", err)
	}
	// Same boot: the directory is gone, but that does not prove the process is.
	recoveryEpoch = func() (platform.RecoveryEpoch, error) { return epochOf("11111111-1111-4111-8111-111111111111"), nil }
	if _, err := RecoverMissingRoots(core.StreamRecoveryConfirmation); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatalf("same boot: %v", err)
	}
	if runs, _ := loadRuns(); len(runs) != 3 {
		t.Fatal("registry changed without proof")
	}
	recoveryEpoch = func() (platform.RecoveryEpoch, error) { return epochOf("22222222-2222-4222-8222-222222222222"), nil }
	message, err := RecoverMissingRoots(core.StreamRecoveryConfirmation)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, missing) {
		t.Fatalf("message %q", message)
	}
	runs, err := loadRuns()
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[0].ID != "b" || runs[1].ID != "c" {
		t.Fatalf("kept %+v", runs)
	}
	// Nothing left to prove: the legacy entry has no epoch and stays.
	if _, err := RecoverMissingRoots(core.StreamRecoveryConfirmation); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatalf("legacy entry: %v", err)
	}
}

func TestRecoverMissingRootsRefusesNextToLiveClient(t *testing.T) {
	isolateUser(t)
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, AdapterLauncher{}) }()
	waitSocket(t, root)
	if _, err := RecoverMissingRoots(core.StreamRecoveryConfirmation); err == nil {
		t.Fatal("ran next to a live client")
	}
	cancel()
	<-done
}

func TestDesktopRootExplainsMissingDirectory(t *testing.T) {
	isolateUser(t)
	missing := filepath.Join(t.TempDir(), "gone")
	if err := saveRuns([]recordedRun{{ID: "a", Root: missing}}); err != nil {
		t.Fatal(err)
	}
	if _, err := DesktopRoot(); err == nil || !strings.Contains(err.Error(), "recover-missing-root") {
		t.Fatalf("error %v", err)
	}
}
