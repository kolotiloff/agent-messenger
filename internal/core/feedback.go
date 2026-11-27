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

// FeedbackCount is how many remarks this computer holds, and how many of
// them were in no export yet.
func (s *Store) FeedbackCount() (all, fresh int, err error) {
	err = s.db.QueryRow(`SELECT COUNT(*), COUNT(CASE WHEN exported_at='' THEN 1 END) FROM feedback`).Scan(&all, &fresh)
	return all, fresh, err
}

// FeedbackList returns the remarks, oldest first, without pictures.
func (s *Store) FeedbackList() ([]FeedbackItem, error) {
	return s.feedbackList(false, "")
}

func (s *Store) feedbackList(freshOnly bool, project string) ([]FeedbackItem, error) {
	query := `SELECT id,created_at,author,project,text,version,image IS NOT NULL FROM feedback WHERE (?='' OR project=?)`
	if freshOnly {
		query += ` AND exported_at=''`
	}
	rows, err := s.db.Query(query+` ORDER BY created_at,id`, project, project)
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

// ExportFeedback writes the remarks that were in no export yet into one zip
// file in dir: feedback.md with the texts and the pictures beside it, and
// marks them exported. With no new remarks it writes all of them again
// (owner, 2026-11-16: each export repeated every old remark). Nothing is
// deleted. It returns the file's path, how many remarks it holds and
// whether they are only the new ones.
func (s *Store) ExportFeedback(dir string, at time.Time) (string, int, bool, error) {
	return s.exportFeedback(dir, at, "")
}

// ExportProjectFeedback exports only remarks tagged with the coordinator's project.
func (s *Store) ExportProjectFeedback(dir string, at time.Time, project string) (string, int, bool, error) {
	if !ValidProjectName(project) {
		return "", 0, false, fmt.Errorf("%w: invalid project name", ErrRejected)
	}
	return s.exportFeedback(dir, at, project)
}

func (s *Store) exportFeedback(dir string, at time.Time, project string) (string, int, bool, error) {
	list, err := s.feedbackList(true, project)
	if err != nil {
		return "", 0, false, err
	}
	fresh := len(list) > 0
	if !fresh {
		if list, err = s.feedbackList(false, project); err != nil {
			return "", 0, false, err
		}
	}
	if len(list) == 0 {
		return "", 0, false, fmt.Errorf("%w: there are no remarks yet", ErrRejected)
	}
	path, err := s.writeFeedback(dir, at, list, fresh, project != "")
	if err != nil {
		return "", 0, false, err
	}
	if fresh {
		stamp := now()
		if err := s.tx(func(tx *sql.Tx) error {
			for _, item := range list {
				if _, err := tx.Exec(`UPDATE feedback SET exported_at=? WHERE id=? AND exported_at=''`, stamp, item.ID); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return "", 0, false, err
		}
	}
	return path, len(list), fresh, nil
}

func (s *Store) writeFeedback(dir string, at time.Time, list []FeedbackItem, fresh, scoped bool) (string, error) {
	var computer string
	if err := s.tx(func(tx *sql.Tx) error {
		var err error
		computer, err = computerName(tx)
		return err
	}); err != nil {
		return "", err
	}
	name := strings.Trim(unsafeName.ReplaceAllString(computer, "-"), "-")
	if name == "" {
		name = "computer"
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	base := fmt.Sprintf("AgentMessenger-feedback-%s-%s", name, at.Local().Format("2006-01-02-1504"))
	var f *os.File
	var err error
	if scoped {
		f, err = os.CreateTemp(dir, base+"-*.zip")
	} else {
		f, err = os.OpenFile(filepath.Join(dir, base+".zip"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	}
	if err != nil {
		return "", err
	}
	z := zip.NewWriter(f)
	var md strings.Builder
	which := "all remarks of the journal (none was new since the last export)"
	if fresh {
		which = "new since the last export"
	}
	fmt.Fprintf(&md, "# Kotel feedback\n\nComputer: %s. Exported %s. Remarks: %d, %s.\n", computer, at.UTC().Format(time.RFC3339), len(list), which)
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
			return "", err
		}
		file := "images/" + item.ID + feedbackImageTypes[kind]
		w, err := z.Create(file)
		if err == nil {
			_, err = w.Write(image)
		}
		if err != nil {
			z.Close()
			f.Close()
			return "", err
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
		return "", err
	}
	return f.Name(), nil
}

// ClearExportedFeedback deletes the remarks that already went into an
// export, once the owner has passed the file on (owner, 2026-11-16: "I gave
// you everything, why keep it"). Remarks written after the export stay. It
// returns how many were deleted and how many remain.
func (s *Store) ClearExportedFeedback() (deleted, left int, err error) {
	return s.clearExportedFeedback("")
}

// ClearExportedProjectFeedback deletes only exported remarks of this project.
func (s *Store) ClearExportedProjectFeedback(project string) (int, int, error) {
	if !ValidProjectName(project) {
		return 0, 0, fmt.Errorf("%w: invalid project name", ErrRejected)
	}
	return s.clearExportedFeedback(project)
}

func (s *Store) clearExportedFeedback(project string) (deleted, left int, err error) {
	err = s.tx(func(tx *sql.Tx) error {
		r, err := tx.Exec(`DELETE FROM feedback WHERE exported_at<>'' AND (?='' OR project=?)`, project, project)
		if err != nil {
			return err
		}
		n, _ := r.RowsAffected()
		deleted = int(n)
		return tx.QueryRow(`SELECT COUNT(*) FROM feedback WHERE (?='' OR project=?)`, project, project).Scan(&left)
	})
	return deleted, left, err
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
