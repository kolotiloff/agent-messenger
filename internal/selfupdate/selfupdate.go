// Package selfupdate updates the program on Windows and Linux from the same
// signed channel the Mac app uses (D-048). It replaces only the program file
// the person started (or, for a program installed with `messenger install`,
// installs the new version next to the old one). A running background client
// keeps its version until it is restarted, as on the Mac.
package selfupdate

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

// maxProgram bounds the unpacked program.
const maxProgram = 256 << 20

// Status is what the window shows.
type Status struct {
	// State: unsupported, idle, checking, available, installing, installed, failed.
	State   string `json:"state"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Updater checks the channel and installs a verified update.
type Updater struct {
	Channel    updatefeed.Channel
	Executable string // the program file the person started, symlinks resolved
	Running    int    // build number of the running program
	// Sync downloads newer signed metadata and archive into Channel, or does
	// nothing. Replaced in tests.
	Sync func(ctx context.Context) error
	// Install puts a verified program into an installation made by
	// `messenger install`; nil means the program is a plain file.
	Install func(program string, info buildinfo.Info) (string, error)

	mu        sync.Mutex
	status    Status
	installed string // path of the new program after Apply
}

// Build returns the build number of a version like 0.0.0-preview.33.
func Build(version string) int {
	i := strings.LastIndex(version, ".")
	if i < 0 {
		return 0
	}
	n, _ := strconv.Atoi(version[i+1:])
	return n
}

// Status returns the current state.
func (u *Updater) Status() Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.status.State == "" {
		return u.idle()
	}
	return u.status
}

func (u *Updater) supported() bool {
	return u.Channel.PublicKey != "" && u.Running > 0 && u.Executable != ""
}

// idle reports an update that is already downloaded and verified, if any.
func (u *Updater) idle() Status {
	if !u.supported() {
		return Status{State: "unsupported"}
	}
	if m, err := u.Channel.Load(); err == nil && m.Build > u.Running {
		return Status{State: "available", Version: m.Version}
	}
	return Status{State: "idle"}
}

// Check asks GitHub for a newer signed release and downloads it. A failure
// is reported as failed, never as "no update".
func (u *Updater) Check(ctx context.Context) Status {
	u.mu.Lock()
	if !u.supported() {
		u.status = Status{State: "unsupported"}
		u.mu.Unlock()
		return u.Status()
	}
	if u.status.State == "checking" || u.status.State == "installing" || u.status.State == "installed" {
		s := u.status
		u.mu.Unlock()
		return s
	}
	u.status = Status{State: "checking"}
	u.mu.Unlock()
	err := u.Sync(ctx)
	u.mu.Lock()
	defer u.mu.Unlock()
	if err != nil && !errors.Is(err, updatefeed.ErrDownloadBusy) {
		u.status = Status{State: "failed", Error: "не удалось проверить обновления"}
		if s := u.idle(); s.State == "available" {
			u.status = s
		}
		return u.status
	}
	u.status = u.idle()
	return u.status
}

// Apply verifies the downloaded update again and replaces the program. It
// never runs while a check or another install is going on.
func (u *Updater) Apply(ctx context.Context) (Status, error) {
	u.mu.Lock()
	if u.status.State == "checking" || u.status.State == "installing" {
		u.mu.Unlock()
		return u.Status(), errors.New("обновление уже проверяется или устанавливается")
	}
	if u.status.State == "installed" {
		s := u.status
		u.mu.Unlock()
		return s, nil
	}
	u.status = Status{State: "installing"}
	u.mu.Unlock()
	version, path, err := u.apply(ctx)
	u.mu.Lock()
	defer u.mu.Unlock()
	if err != nil {
		u.status = u.idle()
		if u.status.State != "available" {
			u.status = Status{State: "failed", Error: "обновление не установлено"}
		}
		return u.status, err
	}
	u.installed = path
	u.status = Status{State: "installed", Version: version}
	return u.status, nil
}

// Installed is the program to start after a successful Apply.
func (u *Updater) Installed() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.installed
}

func (u *Updater) apply(ctx context.Context) (string, string, error) {
	if !u.supported() {
		return "", "", errors.New("обновление из окна недоступно в этой сборке")
	}
	m, err := u.Channel.Load()
	if err != nil {
		return "", "", fmt.Errorf("нет проверенного обновления: %w", err)
	}
	if m.Build <= u.Running {
		return "", "", errors.New("установлена та же или более новая версия")
	}
	program, err := u.program(m)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Dir(u.Executable)
	staged := filepath.Join(dir, "."+filepath.Base(u.Executable)+".update")
	if runtime.GOOS == "windows" {
		staged += ".exe"
	}
	_ = os.Remove(staged)
	if err := writeProgram(staged, program); err != nil {
		return "", "", fmt.Errorf("не удалось записать новую версию рядом с программой: %w", err)
	}
	defer os.Remove(staged)
	info, err := checkProgram(ctx, staged, m)
	if err != nil {
		return "", "", err
	}
	if u.Install != nil {
		path, err := u.Install(staged, info)
		if err != nil {
			return "", "", err
		}
		return m.Version, path, nil
	}
	if err := replace(u.Executable, staged); err != nil {
		return "", "", err
	}
	return m.Version, u.Executable, nil
}

// program reads the channel's archive, checks its size, checksum and
// signature, and returns the one program file inside it.
func (u *Updater) program(m updatefeed.Manifest) ([]byte, error) {
	path := filepath.Join(u.Channel.Directory, m.SHA256+".zip")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != m.Length {
		return nil, errors.New("архив обновления отсутствует или изменён")
	}
	raw, err := os.ReadFile(path)
	if err != nil || int64(len(raw)) != m.Length {
		return nil, errors.New("архив обновления не прочитан")
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != m.SHA256 {
		return nil, errors.New("контрольная сумма архива не совпадает")
	}
	key, err := base64.StdEncoding.DecodeString(u.Channel.PublicKey)
	sig, sigErr := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || sigErr != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, raw, sig) {
		return nil, errors.New("подпись архива обновления не совпадает")
	}
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, errors.New("архив обновления повреждён")
	}
	want := ProgramName(runtime.GOOS)
	if len(z.File) != 1 || z.File[0].Name != want || !z.File[0].Mode().IsRegular() || z.File[0].UncompressedSize64 > maxProgram {
		return nil, errors.New("в архиве обновления неожиданное содержимое")
	}
	r, err := z.File[0].Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, maxProgram+1))
	if err != nil || len(data) > maxProgram || uint64(len(data)) != z.File[0].UncompressedSize64 {
		return nil, errors.New("архив обновления повреждён")
	}
	return data, nil
}

// ProgramName is the name of the program inside an update archive.
func ProgramName(goos string) string {
	if goos == "windows" {
		return "messenger.exe"
	}
	return "messenger"
}

func writeProgram(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// checkProgram runs the new program's `version` and requires the exact
// version, revision, system and architecture of the signed manifest.
func checkProgram(ctx context.Context, path string, m updatefeed.Manifest) (buildinfo.Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "version")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return buildinfo.Info{}, fmt.Errorf("новая версия не запустилась: %w", err)
	}
	var info buildinfo.Info
	if json.Unmarshal(out, &info) != nil || info.Version != m.Version || info.Revision != m.Revision || info.OS != m.OS || info.Arch != m.Arch {
		return buildinfo.Info{}, errors.New("новая версия не совпадает с подписанным описанием")
	}
	return info, nil
}
