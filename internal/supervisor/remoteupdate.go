package supervisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/core"
)

// Updating this computer at another computer's request (T-078, D-100). The
// client answers a request at once: it refuses while any task is not
// finished, on a Mac (Sparkle updates it in its own app), when it does not
// know the program file the owner installed, or while another update is
// going on. Otherwise it answers "updating" and starts the update helper
// from the program file: the helper installs the newest signed update with
// the same updater the window uses and restarts the background client from
// the installed program. The helper writes its result to a file; whichever
// client runs then (the old one or the new one) sends it and removes the
// file. Nothing is retried on its own: a helper that never reported is
// answered as failed after updateMarkerAge. The helper never touches
// active-runs.json or a reservation.

const (
	// ProgramFile, in the client's data folder, names the program file the
	// owner installed. The window writes it at each start on Windows and
	// Linux: the background client runs from a copy of the program.
	ProgramFile = "program.json"
	// RemoteUpdateResult is the helper's final result.
	RemoteUpdateResult = "remote-update.json"
	// remoteUpdateMarker says an update helper was started and has not
	// reported yet; it holds the request.
	remoteUpdateMarker = "remote-update.running"
)

// updateMarkerAge is how long a helper may take before its request is
// answered as failed.
var updateMarkerAge = 30 * time.Minute

// updateOS is the system the client runs on; replaced in tests.
var updateOS = runtime.GOOS

// RemoteUpdate is one request to update this computer and, in the result
// file, its outcome.
type RemoteUpdate struct {
	Broker  string `json:"broker"`
	Peer    string `json:"peer"`
	ID      string `json:"id"`
	State   string `json:"state,omitempty"`
	Version string `json:"version,omitempty"`
	Text    string `json:"text,omitempty"`
}

// UpdateStarter starts the update helper from program, detached from the
// client, for one request.
type UpdateStarter func(program string, job RemoteUpdate) error

// Refusals of a request to update this computer.
const (
	updateRefusedMac         = "Этот Mac обновляется в самой программе: откройте Messenger на нём и установите обновление там."
	updateRefusedOS          = "Обновление по сети не поддерживается на этой системе."
	updateRefusedUnsupported = "Этот фоновый клиент не поддерживает обновление по сети: откройте окно программы на том компьютере и повторите."
	updateRefusedProgram     = "Откройте окно программы на том компьютере один раз и повторите."
	updateRefusedBusy        = "Обновление на том компьютере уже идёт; дождитесь его результата."
)

func updateRefusedTasks(n int) string {
	return fmt.Sprintf("На компьютере идут задачи (%d). Обновите, когда они закончатся.", n)
}

// ProgramPath returns the program file the window last recorded, or "" if
// it is unknown or no longer a regular file.
func ProgramPath(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, ProgramFile))
	if err != nil || len(raw) > 64<<10 {
		return ""
	}
	var in struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(raw, &in) != nil || in.Path == "" || !filepath.IsAbs(in.Path) {
		return ""
	}
	if fi, err := os.Stat(in.Path); err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	return in.Path
}

// WriteProgramPath records the program file the owner installed.
func WriteProgramPath(root, program string) error {
	if !filepath.IsAbs(program) {
		return errors.New("program path is not absolute")
	}
	raw, err := json.Marshal(struct {
		Path string `json:"path"`
	}{program})
	if err != nil {
		return err
	}
	return writePrivate(filepath.Join(root, ProgramFile), raw)
}

// FinishRemoteUpdate is the helper's last step: it records the result for
// the client to send and clears the marker of a running update.
func FinishRemoteUpdate(root string, result RemoteUpdate) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err := writePrivate(filepath.Join(root, RemoteUpdateResult), raw); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(root, remoteUpdateMarker))
	return nil
}

// writePrivate replaces path with a new 0600 file in one step.
func writePrivate(path string, raw []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	name := f.Name()
	if err := f.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		f.Close()
		os.Remove(name)
		return err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		os.Remove(name)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

func readUpdateFile(path string) (RemoteUpdate, bool) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > 64<<10 {
		return RemoteUpdate{}, false
	}
	var r RemoteUpdate
	if json.Unmarshal(raw, &r) != nil || r.Broker == "" || r.Peer == "" || r.ID == "" {
		return RemoteUpdate{}, false
	}
	return r, true
}

// updateBusy reports an update helper that has not reported yet, or a
// result not sent yet.
func (s *Supervisor) updateBusy() bool {
	for _, name := range []string{remoteUpdateMarker, RemoteUpdateResult} {
		if _, err := os.Lstat(filepath.Join(s.root, name)); err == nil {
			return true
		}
	}
	return false
}

// answerUpdateRequests answers the other computers' requests to update
// this computer. It returns a problem for the window, or "".
func (s *Supervisor) answerUpdateRequests(broker string) string {
	if p := s.sendUpdateResult(); p != "" {
		return p
	}
	reqs, err := s.store.NetUpdateRequests(broker)
	if err != nil {
		return "запросы обновления не читаются: " + err.Error()
	}
	for _, r := range reqs {
		refusal := s.updateRefusal()
		if refusal != "" {
			if err := s.store.ReplyNetUpdate(r, core.UpdateRefused, "", refusal); err != nil {
				return "ответ на запрос обновления не записан: " + err.Error()
			}
			continue
		}
		program := ProgramPath(s.root)
		if program == "" {
			if err := s.store.ReplyNetUpdate(r, core.UpdateRefused, "", updateRefusedProgram); err != nil {
				return "ответ на запрос обновления не записан: " + err.Error()
			}
			continue
		}
		job := RemoteUpdate{Broker: r.Broker, Peer: r.Peer, ID: r.ID}
		raw, _ := json.Marshal(job)
		marker := filepath.Join(s.root, remoteUpdateMarker)
		f, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			if err := s.store.ReplyNetUpdate(r, core.UpdateRefused, "", updateRefusedBusy); err != nil {
				return "ответ на запрос обновления не записан: " + err.Error()
			}
			continue
		}
		_, werr := f.Write(raw)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			_ = os.Remove(marker)
			if err := s.store.ReplyNetUpdate(r, core.UpdateFailed, "", "обновление не начато: "+werr.Error()); err != nil {
				return "ответ на запрос обновления не записан: " + err.Error()
			}
			continue
		}
		// The answer is recorded before the helper starts, so the final
		// result always comes after it.
		if err := s.store.ReplyNetUpdate(r, core.UpdateUpdating, "", ""); err != nil {
			_ = os.Remove(marker)
			return "ответ на запрос обновления не записан: " + err.Error()
		}
		if err := s.startUpdate(program, job); err != nil {
			_ = os.Remove(marker)
			if err := s.store.FinishNetUpdate(r.Broker, r.Peer, r.ID, core.UpdateFailed, "", "помощник обновления не запустился: "+err.Error()); err != nil {
				return "результат обновления не записан: " + err.Error()
			}
		}
	}
	return ""
}

// updateRefusal is the reason this computer cannot be updated now, or "".
func (s *Supervisor) updateRefusal() string {
	switch {
	case updateOS == "darwin":
		return updateRefusedMac
	case updateOS != "windows" && updateOS != "linux":
		return updateRefusedOS
	case s.startUpdate == nil:
		return updateRefusedUnsupported
	}
	if n, err := s.store.ActiveWork(); err != nil {
		return "задачи на компьютере не читаются: " + err.Error()
	} else if n > 0 {
		return updateRefusedTasks(n)
	}
	if s.updateBusy() {
		return updateRefusedBusy
	}
	return ""
}

// sendUpdateResult sends the helper's final result, if it wrote one, and
// answers as failed a helper that never reported.
func (s *Supervisor) sendUpdateResult() string {
	resultPath := filepath.Join(s.root, RemoteUpdateResult)
	marker := filepath.Join(s.root, remoteUpdateMarker)
	if _, err := os.Lstat(resultPath); err == nil {
		r, ok := readUpdateFile(resultPath)
		if ok && r.State != core.UpdateUpdating {
			if err := s.store.FinishNetUpdate(r.Broker, r.Peer, r.ID, r.State, r.Version, r.Text); err != nil && !errors.Is(err, core.ErrRejected) {
				return "результат обновления не отправлен: " + err.Error()
			}
		}
		_ = os.Remove(resultPath)
		if m, mok := readUpdateFile(marker); mok && m.ID == r.ID {
			_ = os.Remove(marker)
		}
		return ""
	}
	fi, err := os.Lstat(marker)
	if err != nil || time.Since(fi.ModTime()) < updateMarkerAge {
		return ""
	}
	if m, ok := readUpdateFile(marker); ok {
		why := "помощник обновления не сообщил результат; проверьте версию программы на том компьютере"
		if err := s.store.FinishNetUpdate(m.Broker, m.Peer, m.ID, core.UpdateFailed, "", why); err != nil && !errors.Is(err, core.ErrRejected) {
			return "результат обновления не отправлен: " + err.Error()
		}
	}
	_ = os.Remove(marker)
	return ""
}
