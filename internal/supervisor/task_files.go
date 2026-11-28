package supervisor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/kolotiloff/agent-messenger/internal/core"
)

const maxTaskFileBytes = 1 << 20

type taskFileView struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Format    string `json:"format,omitempty"`
	Text      string `json:"text,omitempty"`
	Data      []byte `json:"data,omitempty"`
	CanSave   bool   `json:"can_save,omitempty"`
	CanReveal bool   `json:"can_reveal,omitempty"`
	CanEdit   bool   `json:"can_edit,omitempty"`
}

// TaskFile reads files from the executor's registered project on this computer.
// A remote task's report never resolves against a same-named local checkout.
func (s *Supervisor) TaskFile(ctx context.Context, id, path, action string) (any, error) {
	if id == "" || len(id) > 128 || path == "" || len(path) > 4096 || strings.ContainsAny(path, "\x00\r\n") {
		return nil, core.ErrRejected
	}
	if action != "preview" && action != "download" && action != "reveal" && action != "edit" {
		return nil, core.ErrRejected
	}
	t, err := s.store.TaskByID(id)
	if err != nil || t.Folder == "" {
		return nil, core.ErrRejected
	}
	computer, err := s.store.LocalComputer()
	if err != nil || t.Computer != computer {
		return nil, core.ErrRejected
	}
	root, err := filepath.EvalSymlinks(t.Folder)
	if err != nil {
		return nil, core.ErrRejected
	}
	projects, err := s.store.Projects()
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		if project.Name == t.Project && project.Folder != "" {
			current, err := filepath.EvalSymlinks(project.Folder)
			if err == nil && current == root {
				return readTaskFile(ctx, root, path, action)
			}
			break
		}
	}
	return nil, core.ErrRejected
}

func readTaskFile(ctx context.Context, root, path, action string) (taskFileView, error) {
	if filepath.IsAbs(path) {
		var err error
		path, err = filepath.Rel(root, path)
		if err != nil {
			return taskFileView{}, core.ErrRejected
		}
	}
	path = filepath.FromSlash(path)
	if !filepath.IsLocal(path) || path == "." || strings.Contains(path, ":") {
		return taskFileView{}, core.ErrRejected
	}
	full := filepath.Join(root, path)
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() {
		return taskFileView{}, core.ErrRejected
	}
	file, err := openTaskFile(root, path)
	if err != nil {
		return taskFileView{}, core.ErrRejected
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return taskFileView{}, core.ErrRejected
	}

	view := taskFileView{Name: filepath.Base(path), Path: full, CanSave: opened.Size() <= maxTaskFileBytes}
	view.CanReveal = runtime.GOOS == "darwin" || runtime.GOOS == "windows" || (runtime.GOOS == "linux" && commandAvailable("xdg-open"))
	if action == "reveal" && view.CanReveal {
		return view, revealTaskFile(ctx, full)
	}
	if action == "reveal" || (!view.CanSave && action != "preview") {
		return taskFileView{}, core.ErrRejected
	}
	if !view.CanSave {
		return view, nil
	}
	src, err := io.ReadAll(io.LimitReader(file, maxTaskFileBytes+1))
	if err != nil {
		return taskFileView{}, core.ErrRejected
	}
	if len(src) > maxTaskFileBytes {
		view.CanSave = false
		if action == "preview" {
			return view, nil
		}
		return taskFileView{}, core.ErrRejected
	}
	if action == "download" {
		view.Data = src
		return view, nil
	}
	if utf8.Valid(src) && !bytes.ContainsRune(src, 0) {
		view.Format, view.Text = "text", strings.TrimPrefix(string(src), "\ufeff")
		switch strings.ToLower(filepath.Ext(path)) {
		case ".md", ".markdown", ".mdown":
			view.Format = "markdown"
		}
		view.CanEdit = runtime.GOOS == "darwin"
	}
	if action == "edit" {
		if !view.CanEdit {
			return taskFileView{}, core.ErrRejected
		}
		return view, exec.CommandContext(ctx, "/usr/bin/open", "-t", full).Run()
	}
	return view, nil
}

func commandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func revealTaskFile(ctx context.Context, path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.CommandContext(ctx, "/usr/bin/open", "-R", path).Run()
	case "windows":
		return exec.CommandContext(ctx, "explorer.exe", filepath.Dir(path)).Run()
	case "linux":
		return exec.CommandContext(ctx, "xdg-open", filepath.Dir(path)).Run()
	default:
		return fmt.Errorf("%w: file manager unavailable", core.ErrRejected)
	}
}
