package mcp

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/core"
)

// taskAttachmentFiles exists only in the receipt-scoped worker tool server.
type taskAttachmentFiles struct {
	dir   string
	paths []string
}

func (files *taskAttachmentFiles) close() {
	if files.dir != "" {
		_ = os.RemoveAll(files.dir)
	}
}

func (files *taskAttachmentFiles) materialize(attachments []core.TaskAttachment) ([]string, error) {
	if files.dir != "" {
		return files.paths, nil
	}
	dir, err := os.MkdirTemp("", "kotel-attachments-")
	if err != nil {
		return nil, err
	}
	ready := false
	defer func() {
		if !ready {
			_ = os.RemoveAll(dir)
		}
	}()

	tops := make(map[string]bool)
	for _, file := range attachments {
		name := filepath.FromSlash(file.Path)
		top, _, _ := strings.Cut(file.Path, "/")
		tops[top] = true
		if file.Dir {
			err = os.MkdirAll(filepath.Join(dir, name), 0700)
		} else {
			err = os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0700)
			if err == nil {
				err = writeTaskAttachment(filepath.Join(dir, name), file.Data)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	for top := range tops {
		files.paths = append(files.paths, filepath.Join(dir, top))
	}
	sort.Strings(files.paths)
	files.dir = dir
	ready = true
	return files.paths, nil
}

func writeTaskAttachment(name string, data []byte) error {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
