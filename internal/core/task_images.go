package core

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

// T-175: a worker's pictures (a screenshot of the scene, a chart) reach the
// owner and the coordinator. The worker names image files with its report or
// question; Messenger reads them once, checks that they are pictures by
// their content, and keeps copies with the run. They are published with the
// run's report or question, after verification, like the text.

const (
	// MaxTaskImages is how many pictures one report or question may carry.
	MaxTaskImages = 4
	// MaxTaskImage is the largest picture accepted, in bytes.
	MaxTaskImage = 5 << 20
)

// TaskImageTypes are the picture types accepted, by detected content type.
var TaskImageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

const taskImagesSchema = `
CREATE TABLE IF NOT EXISTS task_images (
 id TEXT PRIMARY KEY,
 task_id TEXT NOT NULL,
 run_id TEXT NOT NULL DEFAULT '',
 event_seq INTEGER NOT NULL DEFAULT 0,
 seq INTEGER NOT NULL,
 name TEXT NOT NULL,
 type TEXT NOT NULL,
 data BLOB NOT NULL,
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS task_images_run ON task_images(run_id);
CREATE INDEX IF NOT EXISTS task_images_task ON task_images(task_id);
`

// TaskImageFile is one picture a worker attached.
type TaskImageFile struct {
	Name string
	Data []byte
}

// TaskImage describes a kept picture.
type TaskImage struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Size int    `json:"size"`
}

// checkTaskImages refuses what is not a set of pictures; it returns their
// detected types.
func checkTaskImages(images []TaskImageFile) ([]string, error) {
	if len(images) > MaxTaskImages {
		return nil, fmt.Errorf("%w: at most %d pictures", ErrRejected, MaxTaskImages)
	}
	types := make([]string, len(images))
	for i, img := range images {
		if len(img.Data) == 0 || len(img.Data) > MaxTaskImage {
			return nil, fmt.Errorf("%w: picture %s must be 1 byte to %d MB", ErrRejected, img.Name, MaxTaskImage>>20)
		}
		kind := http.DetectContentType(img.Data)
		if !TaskImageTypes[kind] {
			return nil, fmt.Errorf("%w: %s is not a PNG, JPEG, GIF or WebP picture", ErrRejected, img.Name)
		}
		types[i] = kind
	}
	return types, nil
}

// insertTaskImages keeps a run's pictures.
func insertTaskImages(tx *sql.Tx, task, run string, images []TaskImageFile, types []string) error {
	for i, img := range images {
		id, err := randomID()
		if err != nil {
			return err
		}
		name := clip(filepath.Base(strings.TrimSpace(img.Name)), 120)
		if _, err := tx.Exec(`INSERT INTO task_images(id,task_id,run_id,seq,name,type,data,created_at) VALUES (?,?,?,?,?,?,?,?)`,
			"img-"+id[:20], task, run, i+1, name, types[i], img.Data, now()); err != nil {
			return err
		}
	}
	return nil
}

// RunImages lists a run's pictures.
func (s *Store) RunImages(run string) ([]TaskImage, error) {
	return queryImages(s.db, `SELECT id,name,type,length(data) FROM task_images WHERE run_id=? ORDER BY seq`, run)
}

// TaskImages lists the pictures of a task's published reports and
// questions, newest run first, up to limit.
func (s *Store) TaskImages(task string, limit int) ([]TaskImage, error) {
	if s.split != nil {
		scoped, err := s.storeForTask(task)
		if err != nil {
			return nil, err
		}
		return scoped.TaskImages(task, limit)
	}
	return queryImages(s.db, `SELECT i.id,i.name,i.type,length(i.data) FROM task_images i
 LEFT JOIN task_runs r ON r.id=i.run_id
 WHERE i.task_id=? AND (i.run_id='' OR r.state='finished')
 ORDER BY COALESCE(r.ordinal,0) DESC, i.event_seq DESC, i.seq LIMIT ?`, task, limit)
}

type querier interface {
	Query(string, ...any) (*sql.Rows, error)
}

func queryImages(q querier, query string, args ...any) ([]TaskImage, error) {
	rows, err := q.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TaskImage
	for rows.Next() {
		var img TaskImage
		if err := rows.Scan(&img.ID, &img.Name, &img.Type, &img.Size); err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, rows.Err()
}

// TaskImageData returns a kept picture and its type.
func (s *Store) TaskImageData(id string) (string, []byte, error) {
	if s.split != nil {
		var kind string
		var data []byte
		err := s.eachProjectStore(func(scoped *Store) error {
			if data != nil {
				return nil
			}
			var err error
			kind, data, err = scoped.taskImageData("", id)
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		})
		if err != nil || data != nil {
			return kind, data, err
		}
		return "", nil, fmt.Errorf("%w: no such picture", ErrRejected)
	}
	return s.TaskImageForTask("", id)
}

// TaskImageForTask opens a picture only from the given task's store.
func (s *Store) TaskImageForTask(task, id string) (string, []byte, error) {
	scoped, err := s.storeForTask(task)
	if err != nil {
		return "", nil, err
	}
	kind, data, err := scoped.taskImageData(task, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, fmt.Errorf("%w: no such picture", ErrRejected)
	}
	return kind, data, err
}

func (s *Store) taskImageData(task, id string) (string, []byte, error) {
	var kind string
	var data []byte
	err := s.db.QueryRow(`SELECT type,data FROM task_images WHERE id=? AND (?='' OR task_id=?)`, id, task, task).Scan(&kind, &data)
	return kind, data, err
}

// imagesNote tells the coordinator a report or question has pictures.
func imagesNote(tx *sql.Tx, run, text string) (string, error) {
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM task_images WHERE run_id=?`, run).Scan(&n); err != nil {
		return "", err
	}
	if n == 0 {
		return text, nil
	}
	return text + fmt.Sprintf("\n[%d picture(s) attached: read them with task_images]", n), nil
}

// RunFolder is the folder a run works in, for reading the files it names.
func (s *Store) RunFolder(run string) (string, error) {
	var folder sql.NullString
	err := s.db.QueryRow(`SELECT t.folder FROM tasks t JOIN task_runs r ON r.task_id=t.id WHERE r.id=?`, run).Scan(&folder)
	return folder.String, err
}
