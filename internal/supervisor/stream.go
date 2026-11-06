package supervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/mcp"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

type streamEvent struct {
	value map[string]any
	err   error
}
type streamExit struct {
	code int
	err  error
}

func (s *Supervisor) runStream(parent context.Context, token string, launcher StreamLauncher) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	s.dispatch.Lock()
	if s.stopping.Load() {
		s.dispatch.Unlock()
		_ = s.store.MarkUncertain(token, "shutdown before stream launch")
		return
	}
	if err := s.store.BeginLaunch(token); err != nil {
		s.dispatch.Unlock()
		_ = s.store.MarkUncertain(token, err.Error())
		return
	}
	if err := s.store.BeginStream(token); err != nil {
		s.dispatch.Unlock()
		_ = s.store.MarkUncertain(token, err.Error())
		return
	}
	defer func() { _ = s.store.StopStream(token, "stream controller stopped; no automatic replay") }()
	spec, err := s.spec(token)
	if err != nil {
		s.dispatch.Unlock()
		return
	}
	secret, err := newRunID()
	if err != nil {
		s.dispatch.Unlock()
		return
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		s.dispatch.Unlock()
		return
	}
	server := &http.Server{Handler: &mcp.StreamHandler{Store: s.store, Run: token, Secret: secret}, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second}
	go func() { _ = server.Serve(ln) }()
	defer server.Close()
	job := &activeJob{token: token, alias: spec.Alias, streamID: token, cancel: cancel}
	s.addJob(job)
	defer s.removeJob(token)
	s.dispatch.Unlock()
	process, err := s.startStream(ctx, launcher, spec, "http://"+ln.Addr().String()+"/mcp", secret)
	s.dispatch.Lock()
	if err != nil {
		if errors.Is(err, errNotStarted) {
			_ = s.store.FailStreamStart(token, err.Error())
		}
		s.dispatch.Unlock()
		return
	}
	obs := process.Observation
	job.pgid = obs.PGID
	if ctx.Err() != nil || s.stopping.Load() {
		s.dispatch.Unlock()
		process.Input.Close()
		process.Output.Close()
		go process.Wait()
		return
	}
	if err := s.store.RecordProcess(token, obs); err != nil {
		s.dispatch.Unlock()
		process.Input.Close()
		process.Output.Close()
		go process.Wait()
		return
	}
	s.dispatch.Unlock()
	defer process.Output.Close()
	defer process.Input.Close()
	events := make(chan streamEvent, 64)
	go func() {
		defer close(events)
		scanner := bufio.NewScanner(process.Output)
		scanner.Buffer(make([]byte, 65536), 4<<20)
		for scanner.Scan() {
			var value map[string]any
			err := json.Unmarshal(scanner.Bytes(), &value)
			select {
			case events <- streamEvent{value, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
		err := scanner.Err()
		if err == nil {
			err = errors.New("CLI output closed")
		}
		select {
		case events <- streamEvent{err: err}:
		case <-ctx.Done():
		}
	}()
	exited := make(chan streamExit, 1)
	go func() { code, err := process.Wait(); exited <- streamExit{code, err} }()
	observed := make(chan struct{})
	go func() {
		defer close(observed)
		record := func(exit streamExit) {
			if exit.err == nil {
				_ = s.store.RecordStreamExit(token, exit.code)
			} else {
				_ = s.store.StopStream(token, "CLI wait failed; MCP revoked")
			}
			cancel()
		}
		select {
		case exit := <-exited:
			record(exit)
		case <-ctx.Done():
			select {
			case exit := <-exited:
				record(exit)
			case <-time.After(200 * time.Millisecond):
			}
		}
	}()
	defer func() {
		_ = s.store.StopStream(token, "stream ending; MCP revoked")
		cancel()
		_ = process.Input.Close()
		_ = server.Close()
		// The observer must finish before Run closes the database. The raw Wait
		// goroutine never accesses it and may outlive the controller.
		<-observed
	}()
	pending := true
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if pending {
			s.dispatch.Lock()
			if ctx.Err() != nil || s.stopping.Load() {
				s.dispatch.Unlock()
				return
			}
			paused, err := s.store.Paused()
			if err != nil {
				s.dispatch.Unlock()
				return
			}
			if !paused {
				owned, err := platform.OwnsObservation(obs)
				if err != nil || !owned {
					s.dispatch.Unlock()
					return
				}
				frame, err := adapters.ClaudeStreamFrame(spec.ConversationID)
				if err == nil {
					_, err = s.store.PrepareStreamSend(token)
				}
				if err == nil {
					err = process.Input.SetWriteDeadline(time.Now().Add(2 * time.Second))
				}
				if err == nil {
					_, err = process.Input.Write(frame)
				}
				s.dispatch.Unlock()
				if err != nil {
					return
				}
				if err := s.readStreamTurn(ctx, events, token, spec, obs); err != nil {
					_ = s.store.StopStream(token, err.Error())
					return
				}
				pending = false
				continue
			}
			s.dispatch.Unlock()
		}
		// Between turns, no prompt, heartbeat or model request is sent. Local queue
		// inspection costs no model tokens. Unexpected model output fails closed.
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok || !benignStreamIdle(event, spec.ConversationID) {
				_ = s.store.StopStream(token, "unexpected activity or EOF while idle")
				return
			}
		case <-ticker.C:
			if pending {
				continue
			}
			s.dispatch.Lock()
			if ctx.Err() != nil || s.stopping.Load() {
				s.dispatch.Unlock()
				return
			}
			_, err := s.store.NextStreamTurn(token)
			s.dispatch.Unlock()
			if err == nil {
				pending = true
			} else if !errors.Is(err, core.ErrEmpty) && !errors.Is(err, core.ErrPaused) {
				return
			}
		}
	}
}

func (s *Supervisor) readStreamTurn(ctx context.Context, events <-chan streamEvent, run string, spec Spec, obs platform.Observation) error {
	timer := time.NewTimer(150 * time.Second)
	defer timer.Stop()
	var collected []map[string]any
	initialized := false
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return errors.New("stream turn timed out; no automatic retry")
		case event, ok := <-events:
			if !ok {
				return errors.New("CLI stream closed")
			}
			if event.err != nil {
				return event.err
			}
			e := event.value
			if id, ok := e["session_id"].(string); ok && id != spec.ConversationID {
				return errors.New("stream event session mismatch")
			}
			if e["type"] == "system" && e["subtype"] == "init" {
				if initialized || e["session_id"] != spec.ConversationID {
					return errors.New("incorrect or duplicate turn init")
				}
				if owned, err := platform.OwnsObservation(obs); err != nil || !owned {
					return errors.New("stream process identity lost")
				}
				if err := s.store.ConfirmStreamIdentity(run, spec.ConversationID); err != nil {
					return err
				}
				initialized = true
			}
			if (e["type"] == "assistant" || e["type"] == "result") && !initialized {
				return errors.New("model event before confirmed init")
			}
			collected = append(collected, e)
			if len(collected) > 10000 {
				return errors.New("too many events in bounded turn")
			}
			if e["type"] == "result" {
				if err := adapters.ValidateClaudeStream(collected, spec.ConversationID); err != nil {
					return fmt.Errorf("stream result rejected: %w", err)
				}
				return s.store.CompleteStreamTurn(run, true)
			}
		}
	}
}

func benignStreamIdle(event streamEvent, sid string) bool {
	if event.err != nil {
		return false
	}
	e := event.value
	if id, ok := e["session_id"].(string); ok && id != sid {
		return false
	}
	// Only documented status notifications are harmless; unknown output is not
	// silently treated as proof of inactivity.
	return e["type"] == "system" && (e["subtype"] == "status" || e["subtype"] == "hook_response")
}
