package core

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Feedback about the program itself (T-135): the owner writes what is wrong
// or missing from the window, with a picture if they like, and the
// coordinator records what it hits. Everything stays on this computer until
// someone exports it as one file and passes it on; nothing is sent anywhere.

const (
	MaxFeedbackText  = 5000
	MaxFeedbackImage = 2 << 20
)

var feedbackImageTypes = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}

const feedbackSchema = `
CREATE TABLE IF NOT EXISTS feedback (
 id TEXT PRIMARY KEY,
 created_at TEXT NOT NULL,
 author TEXT NOT NULL,
 project TEXT NOT NULL DEFAULT '',
 text TEXT NOT NULL,
 version TEXT NOT NULL DEFAULT '',
 image_type TEXT NOT NULL DEFAULT '',
 image BLOB
);`

// FeedbackItem is one recorded remark.
type FeedbackItem struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Author    string `json:"author"`
	Project   string `json:"project,omitempty"`
	Text      string `json:"text"`
	Version   string `json:"version,omitempty"`
	HasImage  bool   `json:"has_image,omitempty"`
}

// AddFeedback records a remark by the owner or the coordinator.
func (s *Store) AddFeedback(author, project, text, version, imageType string, image []byte) (string, error) {
	text = strings.TrimSpace(text)
	if author != AudienceOwner && author != AudienceCoordinator {
		return "", fmt.Errorf("%w: unknown author", ErrRejected)
	}
	if (text == "" && len(image) == 0) || utf8.RuneCountInString(text) > MaxFeedbackText {
		return "", fmt.Errorf("%w: a remark needs 1..%d characters or a picture", ErrRejected, MaxFeedbackText)
	}
	if len(image) > 0 {
		if feedbackImageTypes[imageType] == "" {
			return "", fmt.Errorf("%w: the picture must be PNG, JPEG or WebP", ErrRejected)
		}
		if len(image) > MaxFeedbackImage {
			return "", fmt.Errorf("%w: the picture is larger than %d MB", ErrRejected, MaxFeedbackImage>>20)
		}
	} else {
		imageType = ""
	}
	id, err := randomID()
	if err != nil {
		return "", err
	}
	id = "fb-" + id[:16]
	var blob any
	if len(image) > 0 {
		blob = image
	}
	_, err = s.db.Exec(`INSERT INTO feedback(id,created_at,author,project,text,version,image_type,image) VALUES (?,?,?,?,?,?,?,?)`,
		id, now(), author, clip(project, 256), text, clip(version, 64), imageType, blob)
	return id, err
}

// FeedbackCount is how many remarks this computer holds.
func (s *Store) FeedbackCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM feedback`).Scan(&n)
	return n, err
}

// FeedbackList returns the remarks, oldest first, without pictures.
func (s *Store) FeedbackList() ([]FeedbackItem, error) {
	rows, err := s.db.Query(`SELECT id,created_at,author,project,text,version,image IS NOT NULL FROM feedback ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FeedbackItem{}
	for rows.Next() {
		var f FeedbackItem
		if err := rows.Scan(&f.ID, &f.CreatedAt, &f.Author, &f.Project, &f.Text, &f.Version, &f.HasImage); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9А-Яа-яЁё._-]+`)

// ExportFeedback writes every remark into one zip file in dir: feedback.md
// with the texts and the pictures beside it. It returns the file's path and
// how many remarks it holds.
func (s *Store) ExportFeedback(dir string, at time.Time) (string, int, error) {
	list, err := s.FeedbackList()
	if err != nil {
		return "", 0, err
	}
	if len(list) == 0 {
		return "", 0, fmt.Errorf("%w: there are no remarks yet", ErrRejected)
	}
	var computer string
	if err := s.tx(func(tx *sql.Tx) error {
		var err error
		computer, err = computerName(tx)
		return err
	}); err != nil {
		return "", 0, err
	}
	name := strings.Trim(unsafeName.ReplaceAllString(computer, "-"), "-")
	if name == "" {
		name = "computer"
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", 0, err
	}
	path := filepath.Join(dir, fmt.Sprintf("AgentMessenger-feedback-%s-%s.zip", name, at.Local().Format("2006-01-02-1504")))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", 0, err
	}
	z := zip.NewWriter(f)
	var md strings.Builder
	fmt.Fprintf(&md, "# Agent Messenger feedback\n\nComputer: %s. Exported %s. Remarks: %d.\n", computer, at.UTC().Format(time.RFC3339), len(list))
	for _, item := range list {
		who := "owner"
		if item.Author == AudienceCoordinator {
			who = "coordinator"
		}
		fmt.Fprintf(&md, "\n## %s — %s", item.CreatedAt, who)
		if item.Project != "" {
			fmt.Fprintf(&md, ", project %s", item.Project)
		}
		if item.Version != "" {
			fmt.Fprintf(&md, ", version %s", item.Version)
		}
		fmt.Fprintf(&md, "\n\n%s\n", item.Text)
		if !item.HasImage {
			continue
		}
		var kind string
		var image []byte
		if err := s.db.QueryRow(`SELECT image_type,image FROM feedback WHERE id=?`, item.ID).Scan(&kind, &image); err != nil {
			z.Close()
			f.Close()
			return "", 0, err
		}
		file := "images/" + item.ID + feedbackImageTypes[kind]
		w, err := z.Create(file)
		if err == nil {
			_, err = w.Write(image)
		}
		if err != nil {
			z.Close()
			f.Close()
			return "", 0, err
		}
		fmt.Fprintf(&md, "\n![picture](%s)\n", file)
	}
	w, err := z.Create("feedback.md")
	if err == nil {
		_, err = w.Write([]byte(md.String()))
	}
	if err == nil {
		err = z.Close()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", 0, err
	}
	return path, len(list), nil
}

// FeedbackDir is where an export goes: the user's Downloads folder when it
// exists, otherwise the data directory.
func FeedbackDir(dataRoot string) string {
	if home, err := os.UserHomeDir(); err == nil {
		d := filepath.Join(home, "Downloads")
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d
		}
		if dataRoot == "" {
			return home
		}
	}
	return dataRoot
}
