package selfupdate

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

const revision = "0123456789abcdef0123456789abcdef01234567"

// The test binary plays the new program: `version` prints what the test
// put in SELFUPDATE_FAKE_VERSION.
func TestMain(m *testing.M) {
	if v := os.Getenv("SELFUPDATE_FAKE_VERSION"); v != "" && len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(v)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type fixture struct {
	t       *testing.T
	key     ed25519.PrivateKey
	channel updatefeed.Channel
	exe     string
	program []byte
}

func newFixture(t *testing.T) *fixture {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "AgentMessenger-Test")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	if err := os.WriteFile(exe, []byte("old program"), 0o755); err != nil {
		t.Fatal(err)
	}
	channel := updatefeed.Channel{Directory: filepath.Join(dir, "updates"), PublicKey: base64.StdEncoding.EncodeToString(pub)}
	if err := os.MkdirAll(channel.Directory, 0o700); err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, key: key, channel: channel, exe: exe, program: program}
}

func (f *fixture) archive(name string, data []byte) []byte {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		f.t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		f.t.Fatal(err)
	}
	return b.Bytes()
}

// publish writes a signed manifest and archive for build into the channel,
// the way a sync from GitHub leaves them.
func (f *fixture) publish(build int, archive []byte, archiveKey ed25519.PrivateKey) updatefeed.Manifest {
	digest := sha256.Sum256(archive)
	m := updatefeed.Manifest{Schema: 1, Build: build, Version: fmt.Sprintf("0.0.0-preview.%d", build), Revision: revision,
		OS: runtime.GOOS, Arch: runtime.GOARCH, ClientAPI: updatefeed.ClientAPI, SHA256: hex.EncodeToString(digest[:]),
		Length: int64(len(archive)), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(archiveKey, archive)), Notes: "test"}
	raw, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(f.channel.Directory, "manifest.json"), raw, 0o600); err != nil {
		f.t.Fatal(err)
	}
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(f.key, raw))
	if err := os.WriteFile(filepath.Join(f.channel.Directory, "manifest.sig"), []byte(sig+"\n"), 0o600); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.channel.Directory, m.SHA256+".zip"), archive, 0o600); err != nil {
		f.t.Fatal(err)
	}
	return m
}

func (f *fixture) updater(running int) *Updater {
	return &Updater{Channel: f.channel, Executable: f.exe, Running: running, Sync: func(context.Context) error { return nil }}
}

func (f *fixture) reports(version, rev string) {
	info := buildinfo.Info{Version: version, Revision: rev, OS: runtime.GOOS, Arch: runtime.GOARCH}
	raw, _ := json.Marshal(info)
	f.t.Setenv("SELFUPDATE_FAKE_VERSION", string(raw))
}

func (f *fixture) unchanged() {
	f.t.Helper()
	if raw, _ := os.ReadFile(f.exe); string(raw) != "old program" {
		f.t.Fatal("the program was changed")
	}
	if matches, _ := filepath.Glob(filepath.Join(filepath.Dir(f.exe), ".*update*")); len(matches) != 0 {
		f.t.Fatalf("a staged file was left: %v", matches)
	}
}

func TestApplyReplacesTheProgram(t *testing.T) {
	f := newFixture(t)
	m := f.publish(33, f.archive(ProgramName(runtime.GOOS), f.program), f.key)
	f.reports(m.Version, revision)
	u := f.updater(32)
	if s := u.Status(); s.State != "available" || s.Version != m.Version {
		t.Fatalf("status %+v", s)
	}
	s, err := u.Apply(context.Background())
	if err != nil || s.State != "installed" || s.Version != m.Version || u.Installed() != f.exe {
		t.Fatalf("apply %+v %v %q", s, err, u.Installed())
	}
	if raw, _ := os.ReadFile(f.exe); !bytes.Equal(raw, f.program) {
		t.Fatal("the new program is not in place")
	}
	if runtime.GOOS == "windows" {
		if raw, _ := os.ReadFile(f.exe + ".old"); string(raw) != "old program" {
			t.Fatal("the old program was not kept aside")
		}
		RemoveLeftover(f.exe)
		if _, err := os.Stat(f.exe + ".old"); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("the old program was not removed")
		}
	}
}

func TestApplyRefusesWhatIsNotVerified(t *testing.T) {
	_, otherKey, _ := ed25519.GenerateKey(rand.Reader)
	for _, tc := range []struct {
		name    string
		prepare func(f *fixture)
	}{
		{"archive signed by another key", func(f *fixture) {
			m := f.publish(33, f.archive(ProgramName(runtime.GOOS), f.program), otherKey)
			f.reports(m.Version, revision)
		}},
		{"program reports another version", func(f *fixture) {
			f.publish(33, f.archive(ProgramName(runtime.GOOS), f.program), f.key)
			f.reports("0.0.0-preview.34", revision)
		}},
		{"program reports another revision", func(f *fixture) {
			m := f.publish(33, f.archive(ProgramName(runtime.GOOS), f.program), f.key)
			f.reports(m.Version, strings.Repeat("f", 40))
		}},
		{"unexpected file in the archive", func(f *fixture) {
			m := f.publish(33, f.archive("other", f.program), f.key)
			f.reports(m.Version, revision)
		}},
		{"archive changed after download", func(f *fixture) {
			m := f.publish(33, f.archive(ProgramName(runtime.GOOS), f.program), f.key)
			f.reports(m.Version, revision)
			path := filepath.Join(f.channel.Directory, m.SHA256+".zip")
			raw, _ := os.ReadFile(path)
			raw[len(raw)/2] ^= 0xff
			_ = os.WriteFile(path, raw, 0o600)
		}},
		{"not newer", func(f *fixture) {
			m := f.publish(32, f.archive(ProgramName(runtime.GOOS), f.program), f.key)
			f.reports(m.Version, revision)
		}},
		{"no update", func(f *fixture) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			tc.prepare(f)
			u := f.updater(32)
			if s, err := u.Apply(context.Background()); err == nil || s.State == "installed" {
				t.Fatalf("applied: %+v", s)
			}
			f.unchanged()
		})
	}
}

func TestCheckReportsFailureNotNoUpdate(t *testing.T) {
	f := newFixture(t)
	u := f.updater(32)
	u.Sync = func(context.Context) error { return errors.New("offline") }
	if s := u.Check(context.Background()); s.State != "failed" {
		t.Fatalf("offline %+v", s)
	}
	u.Sync = func(context.Context) error { return nil }
	if s := u.Check(context.Background()); s.State != "idle" {
		t.Fatalf("nothing new %+v", s)
	}
	u.Sync = func(context.Context) error {
		f.publish(33, f.archive(ProgramName(runtime.GOOS), f.program), f.key)
		return nil
	}
	if s := u.Check(context.Background()); s.State != "available" || s.Version != "0.0.0-preview.33" {
		t.Fatalf("new %+v", s)
	}
}

func TestUnsupportedWithoutKeyOrVersion(t *testing.T) {
	f := newFixture(t)
	u := f.updater(0)
	if s := u.Check(context.Background()); s.State != "unsupported" {
		t.Fatalf("%+v", s)
	}
	u = f.updater(32)
	u.Channel.PublicKey = ""
	if _, err := u.Apply(context.Background()); err == nil {
		t.Fatal("applied without a key")
	}
	f.unchanged()
}

func TestInstalledProgramGoesThroughTheInstaller(t *testing.T) {
	f := newFixture(t)
	m := f.publish(33, f.archive(ProgramName(runtime.GOOS), f.program), f.key)
	f.reports(m.Version, revision)
	u := f.updater(32)
	var got buildinfo.Info
	u.Install = func(program string, info buildinfo.Info) (string, error) {
		raw, err := os.ReadFile(program)
		if err != nil || !bytes.Equal(raw, f.program) {
			return "", errors.New("installer got another program")
		}
		got = info
		return "/installed/messenger", nil
	}
	if s, err := u.Apply(context.Background()); err != nil || s.State != "installed" || u.Installed() != "/installed/messenger" {
		t.Fatalf("%+v %v", s, err)
	}
	if got.Version != m.Version || got.Revision != revision {
		t.Fatalf("installer info %+v", got)
	}
	f.unchanged()
}

func TestBuild(t *testing.T) {
	if Build("0.0.0-preview.33") != 33 || Build("dev") != 0 {
		t.Fatal("build numbers")
	}
}
