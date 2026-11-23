package core

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// A cursor describes the transcript the window has already received. Only
// a new item in the latest run can be sent without rebuilding that prefix.
type feedPosition struct {
	Revision int64  `json:"v"`
	Remote   bool   `json:"r,omitempty"`
	Run      string `json:"run,omitempty"`
	Ordinal  int    `json:"o,omitempty"`
	Seq      int64  `json:"s"`
	Append   bool   `json:"a"`
}

type feedCursor struct {
	Task     string       `json:"id"`
	Position feedPosition `json:"p"`
	Count    int          `json:"n"`
	Last     string       `json:"t"`
}

func (c feedCursor) encode() string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (s *Store) feedPosition(id string) (feedPosition, error) {
	var p feedPosition
	err := s.db.QueryRow(`SELECT COALESCE(w.revision,0),t.remote_peer IS NOT NULL,
 COALESCE(r.id,''),COALESCE(r.ordinal,0),
 CASE WHEN t.remote_peer IS NOT NULL THEN COALESCE((SELECT MAX(seq) FROM remote_task_items WHERE task_id=t.id),0)
 ELSE COALESCE((SELECT MAX(seq) FROM task_run_items WHERE run_id=r.id),0) END,
 (t.remote_peer IS NOT NULL OR (COALESCE(r.state,'')='running' AND COALESCE(r.disposition,'')=''))
 FROM tasks t LEFT JOIN window_revisions w ON w.scope='feed_structure' AND w.item=t.id
 LEFT JOIN task_runs r ON r.id=(SELECT n.id FROM task_runs n WHERE n.task_id=t.id ORDER BY n.ordinal DESC LIMIT 1)
 WHERE t.id=?`, id).Scan(&p.Revision, &p.Remote, &p.Run, &p.Ordinal, &p.Seq, &p.Append)
	return p, err
}

// TaskFeedDelta reads only unseen worker items if the cursor still matches
// the structure of the transcript. A reordered or rewritten feed is returned
// in full. Two position reads also guard against a concurrent commit.
func (s *Store) TaskFeedDelta(id, previous string) (TaskFeed, error) {
	if s.split != nil {
		scoped, err := s.storeForTask(id)
		if err != nil {
			return TaskFeed{}, err
		}
		return scoped.TaskFeedDelta(id, previous)
	}
	before, err := s.feedPosition(id)
	if err != nil {
		return s.TaskFeed(id)
	}
	var cursor feedCursor
	raw, err := base64.RawURLEncoding.DecodeString(previous)
	if err == nil && json.Unmarshal(raw, &cursor) == nil && cursor.Task == id && cursor.Count > 0 &&
		cursor.Position.Revision == before.Revision && cursor.Position.Remote == before.Remote &&
		cursor.Position.Run == before.Run && cursor.Position.Seq <= before.Seq && before.Append {
		delta, ok, err := s.appendTaskFeed(id, cursor, before)
		if err != nil || ok {
			return delta, err
		}
	}
	feed, err := s.TaskFeed(id)
	if err != nil || len(feed.Items) == 0 {
		return feed, err
	}
	after, err := s.feedPosition(id)
	if err == nil && before == after {
		feed.Cursor = (feedCursor{id, after, len(feed.Items), feed.Items[len(feed.Items)-1].At}).encode()
	}
	return feed, nil
}

func (s *Store) appendTaskFeed(id string, cursor feedCursor, before feedPosition) (TaskFeed, bool, error) {
	feed, _, err := s.taskFeedHeader(id)
	if err != nil {
		return feed, false, err
	}
	query := `SELECT kind,title,text,description,created_at FROM task_run_items WHERE run_id=? AND seq>? ORDER BY seq`
	key := before.Run
	if before.Remote {
		query = `SELECT kind,title,text,description,created_at FROM remote_task_items WHERE task_id=? AND seq>? ORDER BY seq`
		key = id
	}
	rows, err := s.db.Query(query, key, cursor.Position.Seq)
	if err != nil {
		return feed, false, err
	}
	defer rows.Close()
	feed.Items = []FeedItem{}
	last, err := time.Parse(time.RFC3339Nano, cursor.Last)
	if err != nil {
		return feed, false, nil
	}
	for rows.Next() {
		it := FeedItem{Author: "worker", Run: before.Ordinal, Verified: true}
		if before.Remote {
			it.Run = 0
		}
		if err := rows.Scan(&it.Kind, &it.Title, &it.Text, &it.Description, &it.At); err != nil {
			return feed, false, err
		}
		at, err := time.Parse(time.RFC3339Nano, it.At)
		if err != nil || !at.After(last) {
			return feed, false, nil
		}
		last, cursor.Last = at, it.At
		feed.Items = append(feed.Items, it)
	}
	if err := rows.Err(); err != nil {
		return feed, false, err
	}
	rows.Close()
	after, err := s.feedPosition(id)
	if err != nil || before != after {
		return feed, false, err
	}
	feed.AppendFrom = cursor.Count
	cursor.Count += len(feed.Items)
	cursor.Position = after
	feed.Cursor = cursor.encode()
	return feed, true, nil
}
