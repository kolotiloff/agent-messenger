package main

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
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/distribution"
	"github.com/kolotiloff/agent-messenger/internal/selfupdate"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

func TestInstallPrefix(t *testing.T) {
	prefix := filepath.Join(string(filepath.Separator)+"home", "a", ".local")
	installed := filepath.Join(prefix, "lib", "agent-messenger", "0.0.0-preview.33-abc", "messenger")
	if got := distribution.InstallPrefix(installed); got != prefix {
		t.Fatalf("installed: %q", got)
	}
	for _, plain := range []string{
		filepath.Join(prefix, "Downloads", "AgentMessenger-Linux-x64"),
		filepath.Join(prefix, "lib", "other", "v", "messenger"),
		filepath.Join(prefix, "bin", "agent-messenger", "v", "messenger"),
	} {
		if got := distribution.InstallPrefix(plain); got != "" {
			t.Fatalf("%s: %q", plain, got)
		}
	}
}

func TestUpdatedWindowWaitsForNewClient(t *testing.T) {
	root := shortRoot(t)
	ctx := context.Background()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	program := []byte("installed window")
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	w, err := z.Create(selfupdate.ProgramName(runtime.GOOS))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(program); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(archive.Bytes())
	m := updatefeed.Manifest{Schema: 1, Build: 34, Version: "0.0.0-preview.34", Revision: "0123456789abcdef0123456789abcdef01234567",
		OS: runtime.GOOS, Arch: runtime.GOARCH, ClientAPI: updatefeed.ClientAPI, SHA256: hex.EncodeToString(digest[:]),
		Length: int64(archive.Len()), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, archive.Bytes()))}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, selfupdate.ProgramName(runtime.GOOS))
	for name, data := range map[string][]byte{
		filepath.Base(path): program, m.SHA256 + ".zip": archive.Bytes(),
		"manifest.json": raw, "manifest.sig": []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(key, raw))),
	} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := supervisor.WriteProgramPath(root, path); err != nil {
		t.Fatal(err)
	}
	u := &selfupdate.Updater{Channel: updatefeed.Channel{Directory: root, PublicKey: base64.StdEncoding.EncodeToString(pub)}, Running: 33}
	old := serveFake(t, root, &fakeClient{version: "0.0.0-preview.33"})
	if got := updatedWindowProgram(ctx, root, u); got != "" {
		t.Fatal("reopening before the client is updated", got)
	}
	old.stop()
	if got := updatedWindowProgram(ctx, root, u); got != "" {
		t.Fatal("reopening while the client is down", got)
	}
	fresh := serveFake(t, root, &fakeClient{version: m.Version, paused: true})
	if got := updatedWindowProgram(ctx, root, u); got != path {
		t.Fatalf("relaunch program %q, want %q", got, path)
	}
	if old.shutdowns.Load()+fresh.shutdowns.Load()+fresh.resumes.Load() != 0 {
		t.Fatal("window update changed the background client")
	}
}
