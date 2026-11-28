package core

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MaxTaskAttachmentBytes = 112 << 10
	MaxTaskAttachmentItems = 64
)

// TaskAttachment is a file or empty directory relative to an attachment root.
type TaskAttachment struct {
	Path string `json:"path"`
	Data []byte `json:"data,omitempty"`
	Dir  bool   `json:"dir,omitempty"`
}

const taskAttachmentsSchema = `
CREATE TABLE IF NOT EXISTS task_attachments (
 task_id TEXT PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
 payload BLOB NOT NULL
);
`

func normalizeTaskAttachments(input []TaskAttachment) ([]TaskAttachment, error) {
	if len(input) == 0 {
		return nil, nil
	}
	if len(input) > MaxTaskAttachmentItems {
		return nil, fmt.Errorf("%w: too many attachment entries (maximum %d)", ErrRejected, MaxTaskAttachmentItems)
	}
	files := append([]TaskAttachment(nil), input...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	seen := make(map[string]TaskAttachment, len(files))
	total := 0
	for i, file := range files {
		if !validAttachmentPath(file.Path) || (file.Dir && len(file.Data) != 0) {
			return nil, fmt.Errorf("%w: invalid attachment path %q", ErrRejected, file.Path)
		}
		if len(file.Data) > MaxTaskAttachmentBytes-total {
			return nil, fmt.Errorf("%w: attachments exceed %d KiB", ErrRejected, MaxTaskAttachmentBytes>>10)
		}
		total += len(file.Data)
		if len(file.Data) == 0 {
			files[i].Data = nil
		}
		key := strings.ToLower(file.Path)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("%w: duplicate attachment path %q", ErrRejected, file.Path)
		}
		seen[key] = file
	}
	// Detect spelling collisions in implicit directories as well as files.
	directories := map[string]string{}
	for _, file := range files {
		for parent := path.Dir(file.Path); parent != "."; parent = path.Dir(parent) {
			key := strings.ToLower(parent)
			if prior, ok := seen[key]; ok && (!prior.Dir || prior.Path != parent) {
				return nil, fmt.Errorf("%w: conflicting attachment path %q", ErrRejected, file.Path)
			}
			if prior, ok := directories[key]; ok && prior != parent {
				return nil, fmt.Errorf("%w: conflicting attachment path %q", ErrRejected, file.Path)
			}
			directories[key] = parent
		}
	}
	return files, nil
}

func validAttachmentPath(name string) bool {
	if name == "" || len(name) > 300 || !utf8.ValidString(name) || name == "." || path.Clean(name) != name || path.IsAbs(name) {
		return false
	}
	if strings.ContainsAny(name, "\\:*\"<>?|") || strings.ContainsFunc(name, unicode.IsControl) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if len(part) > 120 || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
		base, _, _ := strings.Cut(strings.ToUpper(part), ".")
		switch base {
		case "CON", "PRN", "AUX", "NUL":
			return false
		}
		if strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT") {
			number := strings.TrimPrefix(strings.TrimPrefix(base, "COM"), "LPT")
			if number != "" && strings.Contains("123456789¹²³", number) {
				return false
			}
		}
	}
	return true
}

func sameTaskAttachments(a, b []TaskAttachment) bool {
	return slices.EqualFunc(a, b, func(a, b TaskAttachment) bool {
		return a.Path == b.Path && a.Dir == b.Dir && bytes.Equal(a.Data, b.Data)
	})
}

func putTaskAttachments(tx *sql.Tx, id string, attachments []TaskAttachment) error {
	if len(attachments) == 0 {
		return nil
	}
	raw, err := json.Marshal(attachments)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO task_attachments(task_id,payload) VALUES (?,?)`, id, raw)
	return err
}

func getTaskAttachments(tx *sql.Tx, id string) ([]TaskAttachment, error) {
	var raw []byte
	err := tx.QueryRow(`SELECT payload FROM task_attachments WHERE task_id=?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []TaskAttachment
	err = json.Unmarshal(raw, &files)
	return files, err
}
