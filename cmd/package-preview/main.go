// package-preview creates a native macOS technical-preview archive, not a release.
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/distribution"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	version := flag.String("version", "", "version such as 0.0.0-preview.1")
	revision := flag.String("revision", "", "full source commit SHA declared by the builder")
	out := flag.String("out", "dist", "output directory")
	goBinary := flag.String("go", "go", "Go executable")
	external := flag.Bool("external-linker", false, "use native C linker (needed for tested Go 1.22.2/macOS combination)")
	flag.Parse()
	if flag.NArg() != 0 || !distribution.ValidBuild(*version, *revision) {
		return fmt.Errorf("versioned preview and full revision are required")
	}
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("preview packaging is currently validated only on macOS")
	}
	if _, err := os.Stat("go.mod"); err != nil {
		return fmt.Errorf("run from the repository root: %w", err)
	}
	dir, err := filepath.Abs(*out)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(dir, ".package-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	binary := filepath.Join(stage, "messenger")
	flags := "-X github.com/kolotiloff/agent-messenger/internal/buildinfo.Version=" + *version + " -X github.com/kolotiloff/agent-messenger/internal/buildinfo.Revision=" + *revision
	cgo := "0"
	if *external {
		flags += " -linkmode=external"
		cgo = "1"
	}
	cmd := exec.Command(*goBinary, "build", "-trimpath", "-buildvcs=false", "-ldflags", flags, "-o", binary, "./cmd/messenger")
	cmd.Env = filteredEnvironment("CGO_ENABLED", "GOOS", "GOARCH")
	cmd.Env = append(cmd.Env, "CGO_ENABLED="+cgo, "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return err
	}
	sign := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", binary)
	sign.Stdout = os.Stdout
	sign.Stderr = os.Stderr
	if err = sign.Run(); err != nil {
		return err
	}
	raw, err := exec.Command(binary, "version").Output()
	if err != nil {
		return fmt.Errorf("built binary did not run: %w", err)
	}
	var info buildinfo.Info
	if err = json.Unmarshal(raw, &info); err != nil {
		return err
	}
	if info.Version != *version || info.Revision != *revision || info.OS != runtime.GOOS || info.Arch != runtime.GOARCH {
		return fmt.Errorf("binary metadata mismatch")
	}
	digest, err := hashFile(binary)
	if err != nil {
		return err
	}
	manifest := struct {
		Build   buildinfo.Info `json:"build"`
		SHA256  string         `json:"binary_sha256"`
		Signing string         `json:"signing"`
		Status  string         `json:"status"`
	}{info, digest, "ad-hoc; not Developer ID or notarized", "technical preview; macOS native architecture only"}
	manifestRaw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	files := map[string]string{"messenger": binary}
	generated := map[string][]byte{"manifest.json": append(manifestRaw, '\n'), "SHA256SUMS": []byte(digest + "  messenger\n"), "INSTALL.txt": []byte(installGuide)}
	files["README.md"] = "README.md"
	for _, root := range []string{"docs", "design"} {
		if err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("package refuses symlink: %s", path)
			}
			files[filepath.ToSlash(path)] = path
			return nil
		}); err != nil {
			return err
		}
	}
	name := "agent-messenger-" + info.Version + "-" + info.OS + "-" + info.Arch + ".tar.gz"
	archive := filepath.Join(dir, name)
	f, err := os.OpenFile(archive, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		f.Close()
		if !complete {
			os.Remove(archive)
		}
	}()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	names := []string{}
	for name := range files {
		names = append(names, name)
	}
	for name := range generated {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data, ok := generated[name]
		if !ok {
			data, err = os.ReadFile(files[name])
			if err != nil {
				return err
			}
		}
		mode := int64(0o644)
		if name == "messenger" {
			mode = 0o755
		}
		if err = tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data)), ModTime: time.Unix(0, 0)}); err != nil {
			return err
		}
		if _, err = tw.Write(data); err != nil {
			return err
		}
	}
	if err = tw.Close(); err != nil {
		return err
	}
	if err = gz.Close(); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	complete = true
	archiveHash, err := hashFile(archive)
	if err != nil {
		return err
	}
	sum, err := os.OpenFile(archive+".sha256", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(sum, archiveHash+"  "+name)
	closeErr := sum.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Println(archive)
	fmt.Println("Technical preview only. No release was published or installed.")
	return nil
}
func filteredEnvironment(keys ...string) []string {
	env := []string{}
	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		drop := false
		for _, k := range keys {
			if key == k {
				drop = true
			}
		}
		if !drop {
			env = append(env, v)
		}
	}
	return env
}
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

const installGuide = `Agent Messenger — macOS technical preview

1. Verify the archive checksum against a trusted source before extraction.
2. In the extracted directory: shasum -a 256 -c SHA256SUMS
3. Read manifest.json and docs/LOCAL_UI.md. Run ./messenger version.
4. Run ./messenger install (or ./messenger install --prefix /absolute/user/path).
5. Add the printed bin directory to PATH yourself if needed. No shell files change.

No sudo, account login, service, model or client starts during installation.
Activation refuses a running client, pending startup or any retained CLI reserve.
Do not delete reserves to force an update. Follow the documented recovery process.
Previous binaries remain under PREFIX/lib/agent-messenger; SQLite lives separately.
Downgrading a binary is not a database rollback and is not accepted as safe recovery.

This archive is locally ad-hoc signed, not Developer ID signed or notarized.
It is a developer preview, not a validated public macOS installer or a four-OS release.
Checksums detect changes; a checksum bundled with an untrusted file is not proof of origin.
Windows/Linux runtime, native shell, broker and complete adapter acceptance remain open.
`
