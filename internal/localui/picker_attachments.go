package localui

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/google/safeopen"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

// readPickedAttachments reads only what the owner selected in the OS dialog.
func readPickedAttachments(selected string) ([]core.TaskAttachment, error) {
	var out []core.TaskAttachment
	readRoot := filepath.Dir(selected)
	remaining := core.MaxTaskAttachmentBytes
	err := filepath.WalkDir(selected, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if len(out) >= core.MaxTaskAttachmentItems {
			return errors.New("В папке больше 64 файлов и подпапок.")
		}
		rel, err := filepath.Rel(readRoot, name)
		if err != nil {
			return err
		}
		item := core.TaskAttachment{Path: filepath.ToSlash(rel), Dir: entry.IsDir()}
		if !item.Dir {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return errors.New("Вложения должны быть обычными файлами или папками без символических ссылок.")
			}
			if info.Size() > int64(remaining) {
				return errors.New("Вложения занимают больше 112 КБ.")
			}
			item.Data, err = readPickedFile(readRoot, rel, remaining)
			if err != nil {
				return err
			}
			remaining -= len(item.Data)
		}
		out = append(out, item)
		return nil
	})
	return out, err
}

func readPickedFile(root, name string, limit int) ([]byte, error) {
	f, err := safeopen.OpenBeneath(root, name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, os.ErrInvalid
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if len(data) > limit {
		return nil, errors.New("Вложения занимают больше 112 КБ.")
	}
	return data, err
}
