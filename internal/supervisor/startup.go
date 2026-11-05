package supervisor

import (
	"context"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// A cancelled startup can still return from an OS call later. Its lease keeps
// every supervisor/recovery entry point away from the registry until that call
// has finished. The controller can stop without waiting for it.
func acquireStartupLease() (*platform.Lock, error) {
	path, err := platform.SupervisorLockFile()
	if err != nil {
		return nil, err
	}
	return platform.Acquire(path + ".startup")
}

// Called while holding the main supervisor lock, before reading the registry.
func checkStartupLease() error {
	lease, err := acquireStartupLease()
	if err != nil {
		return err
	}
	return lease.Release()
}

func (s *Supervisor) startStream(ctx context.Context, launcher StreamLauncher, spec Spec, url, secret string) (StreamProcess, error) {
	lease, err := acquireStartupLease()
	if err != nil {
		return StreamProcess{}, err
	}
	type result struct {
		process StreamProcess
		err     error
	}
	done := make(chan result)
	s.starting.Store(true)
	go func() {
		defer s.starting.Store(false)
		defer lease.Release()
		if ctx.Err() != nil {
			return
		}
		process, err := launcher.StartStream(ctx, spec, url, secret)
		select {
		case done <- result{process, err}:
		case <-ctx.Done():
			if err == nil {
				// No prompt was sent. Never attach a late process to a stopped run.
				process.Input.Close()
				process.Output.Close()
				go process.Wait()
			}
		}
	}()
	select {
	case r := <-done:
		return r.process, r.err
	case <-ctx.Done():
		return StreamProcess{}, ctx.Err()
	}
}
