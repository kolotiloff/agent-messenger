package distribution

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func installFixture(t *testing.T) (string, string, buildinfo.Info) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("Mac installer")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	source := filepath.Join(t.TempDir(), "messenger")
	if err := os.WriteFile(source, []byte("fixture-v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	return source, filepath.Join(home, "prefix"), buildinfo.Info{Version: "0.0.0-preview.1", Revision: "1111111111111111111111111111111111111111", OS: runtime.GOOS, Arch: runtime.GOARCH}
}
func TestInstallUpdateKeepsOldBinaryAndData(t *testing.T) {
	source, prefix, info := installFixture(t)
	root, err := platform.DefaultRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(root, "queue.sqlite")
	if err = os.WriteFile(data, []byte("sentinel-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	link, err := Install(source, prefix, info)
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Install(source, prefix, info); err != nil {
		t.Fatal("idempotent install", err)
	}
	if err = os.WriteFile(source, []byte("fixture-v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	info.Version = "0.0.0-preview.2"
	if _, err = Install(source, prefix, info); err != nil {
		t.Fatal(err)
	}
	current, _ := os.Readlink(link)
	if old == current {
		t.Fatal("version overwritten")
	}
	oldBytes, _ := os.ReadFile(old)
	newBytes, _ := os.ReadFile(link)
	stored, _ := os.ReadFile(data)
	if string(oldBytes) != "fixture-v1" || string(newBytes) != "fixture-v2" || string(stored) != "sentinel-data" {
		t.Fatal("binary or user data damaged")
	}
}
func TestInstallRefusesBusyAndUncertainRegistry(t *testing.T) {
	for _, state := range []string{"controller", "startup", "empty", "null", "broken", "reserved"} {
		t.Run(state, func(t *testing.T) {
			source, prefix, info := installFixture(t)
			path, err := platform.SupervisorLockFile()
			if err != nil {
				t.Fatal(err)
			}
			switch state {
			case "controller", "startup":
				if state == "startup" {
					path += ".startup"
				}
				lock, err := platform.Acquire(path)
				if err != nil {
					t.Fatal(err)
				}
				defer lock.Release()
			default:
				content := map[string]string{"empty": "", "null": "null", "broken": "[", "reserved": "[{}]"}[state]
				if err = os.WriteFile(filepath.Join(filepath.Dir(path), "active-runs.json"), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = Install(source, prefix, info); err == nil {
				t.Fatal("unsafe activation accepted")
			}
			if _, err = os.Lstat(filepath.Join(prefix, "bin", "messenger")); !os.IsNotExist(err) {
				t.Fatal("installation changed")
			}
		})
	}
}
func TestInstallDoesNotReplaceForeignFile(t *testing.T) {
	source, prefix, info := installFixture(t)
	dir := filepath.Join(prefix, "bin")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "messenger")
	if err := os.WriteFile(path, []byte("foreign"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(source, prefix, info); err == nil {
		t.Fatal("foreign file replaced")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "foreign" {
		t.Fatal("foreign file changed")
	}
}

func TestInstallRejectsForeignLinkAndCorruptVersion(t *testing.T) {
	source, prefix, info := installFixture(t)
	dir := filepath.Join(prefix, "bin")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "messenger")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(source, prefix, info); err == nil {
		t.Fatal("foreign symlink replaced")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(source, prefix, info); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(target, []byte("damaged-installed-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err = Install(source, prefix, info); err == nil {
		t.Fatal("corrupt immutable version silently replaced")
	}
	bytes, _ := os.ReadFile(target)
	if string(bytes) != "damaged-installed-binary" {
		t.Fatal("unexpected mutation of damaged version")
	}
}

// T-107: an update installs while the background client holds its lock; the
// running version's file stays as it was.
func TestInstallUpdateWhileTheClientRuns(t *testing.T) {
	source, prefix, info := installFixture(t)
	link, err := Install(source, prefix, info)
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	path, err := platform.SupervisorLockFile()
	if err != nil {
		t.Fatal(err)
	}
	client, err := platform.Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Release()
	if err = os.WriteFile(source, []byte("fixture-v2"), 0o755); err != nil {
		t.Fatal(err)
	}
	info.Version = "0.0.0-preview.2"
	if _, err = Install(source, prefix, info); err == nil {
		t.Fatal("manual install accepted while the client runs")
	}
	installed, err := InstallUpdate(source, prefix, info)
	if err != nil {
		t.Fatal("update refused while the client runs:", err)
	}
	current, _ := os.Readlink(installed)
	if current == old {
		t.Fatal("link not moved to the new version")
	}
	oldBytes, _ := os.ReadFile(old)
	newBytes, _ := os.ReadFile(installed)
	if string(oldBytes) != "fixture-v1" || string(newBytes) != "fixture-v2" {
		t.Fatal("running version changed or new version missing")
	}
	lock, err := platform.Acquire(filepath.Join(filepath.Dir(filepath.Dir(current)), "install.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	info.Version = "0.0.0-preview.3"
	if _, err = InstallUpdate(source, prefix, info); err == nil {
		t.Fatal("second installation at the same time accepted")
	}
}
