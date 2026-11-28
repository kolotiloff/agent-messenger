package supervisor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

type sleepStatus struct {
	OK     bool   `json:"ok"`
	Mode   string `json:"mode"`
	Active bool   `json:"active"`
	Failed bool   `json:"failed"`
}

type awakePolicy struct {
	mu     sync.Mutex
	status sleepStatus
	wake   chan struct{}
}

func (s *Supervisor) SleepMode(mode string) (any, error) {
	p := s.awake
	p.mu.Lock()
	defer p.mu.Unlock()
	if mode == "" {
		return p.status, nil
	}
	if mode != "off" && mode != "working" && mode != "always" {
		return nil, errors.New("неизвестный режим сна")
	}
	if err := saveSleepMode(s.root, mode); err != nil {
		return nil, errors.New("не удалось сохранить режим сна")
	}
	p.status.Mode, p.status.Failed = mode, false
	select {
	case p.wake <- struct{}{}:
	default:
	}
	return p.status, nil
}

func saveSleepMode(root, mode string) error {
	f, err := os.CreateTemp(root, ".sleep-mode-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString(mode)
	err = errors.Join(err, f.Sync(), f.Close())
	if err != nil {
		return err
	}
	return platform.ReplaceFile(f.Name(), filepath.Join(root, "sleep-mode"))
}

func initialAwakePolicy(root string) *awakePolicy {
	mode := "off"
	if raw, err := platform.ReadFile(filepath.Join(root, "sleep-mode")); err == nil && (string(raw) == "working" || string(raw) == "always") {
		mode = string(raw)
	}
	return &awakePolicy{status: sleepStatus{OK: true, Mode: mode}, wake: make(chan struct{}, 1)}
}

func (s *Supervisor) agentWorking() bool {
	s.tasks.mu.Lock()
	working := len(s.tasks.running) != 0
	s.tasks.mu.Unlock()
	return working || len(s.jobList()) != 0
}

type sleepInhibitor struct {
	cancel  context.CancelFunc
	done    chan error
	started time.Time
}

func startSleepInhibitor(ctx context.Context) *sleepInhibitor {
	hold, cancel := context.WithCancel(ctx)
	inhibitor := &sleepInhibitor{cancel: cancel, done: make(chan error, 1), started: time.Now()}
	go func() { inhibitor.done <- platform.HoldAwake(hold) }()
	return inhibitor
}

func (s *Supervisor) runAwake(ctx context.Context) {
	p := s.awake
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var inhibitor *sleepInhibitor
	retryAfter := time.Time{}
	stop := func() {
		if inhibitor != nil {
			inhibitor.cancel()
			<-inhibitor.done
			inhibitor = nil
		}
	}
	defer stop()
	for {
		p.mu.Lock()
		wanted := p.status.Mode == "always" || p.status.Mode == "working" && s.agentWorking()
		p.mu.Unlock()
		switch {
		case !wanted:
			stop()
			retryAfter = time.Time{}
		case inhibitor == nil && !time.Now().Before(retryAfter):
			inhibitor = startSleepInhibitor(ctx)
		}
		var done <-chan error
		if inhibitor != nil {
			done = inhibitor.done
		}
		p.mu.Lock()
		p.status.Active = wanted && inhibitor != nil && time.Since(inhibitor.started) >= time.Second
		if !wanted || p.status.Active {
			p.status.Failed = false
		}
		p.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-p.wake:
			retryAfter = time.Time{}
		case <-done:
			inhibitor.cancel()
			inhibitor = nil
			retryAfter = time.Now().Add(time.Minute)
			p.mu.Lock()
			p.status.Active, p.status.Failed = false, true
			p.mu.Unlock()
		}
	}
}
