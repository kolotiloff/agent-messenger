package core

import (
	"database/sql"
	"fmt"
	"strings"
)

const timelineTextLimit = 16000

type Timeline struct {
	SchemaVersion int            `json:"schema_version"`
	SessionAlias  string         `json:"session_alias"`
	Items         []TimelineItem `json:"items"`
	HasMore       bool           `json:"has_more"`
	NextBefore    string         `json:"next_before,omitempty"`
}

type TimelineItem struct {
	ID            string         `json:"id"`
	Sender        string         `json:"sender"`
	Text          string         `json:"text"`
	TextTruncated bool           `json:"text_truncated"`
	CreatedAt     string         `json:"created_at"`
	State         string         `json:"state"`
	RetryOf       string         `json:"retry_of,omitempty"`
	AttemptAlias  string         `json:"attempt_alias,omitempty"`
	AttemptState  string         `json:"attempt_state"`
	Reply         *TimelineReply `json:"reply"`
	Result        string         `json:"result"`
	ExitCode      *int           `json:"exit_code"`
}

type TimelineReply struct {
	Reply
	TextTruncated bool `json:"text_truncated"`
}

// Timeline returns a page of the selected session's newest requests, with each
// page in insertion order. before is a Messenger request ID, never a CLI ID.
// It reads recorded outcomes only and cannot release ownership or retry work.
func (s *Store) Timeline(alias string, limit int, before string) (Timeline, error) {
	if strings.TrimSpace(alias) == "" || limit < 1 || limit > 100 {
		return Timeline{}, fmt.Errorf("%w: alias and limit 1..100 required", ErrRejected)
	}
	view := Timeline{SchemaVersion: 1, SessionAlias: alias, Items: []TimelineItem{}}
	err := s.tx(func(tx *sql.Tx) error {
		key, _, err := sessionByAlias(tx, alias)
		if err != nil {
			return err
		}
		var cursor int64
		if before != "" {
			if err := tx.QueryRow(`SELECT rowid FROM requests WHERE id=? AND session_key=?`, before, key).Scan(&cursor); err != nil {
				return err
			}
		}
		rows, err := tx.Query(`SELECT q.id,q.sender,substr(q.text,1,?),length(q.text)>?,q.created_at,q.state,COALESCE(q.retry_of,''),
			COALESCE(a.alias,''),COALESCE(a.state,'none'),
			r.id,r.sender,r.recipient,substr(r.text,1,?),COALESCE(length(r.text)>?,0),
			v.attempt_token IS NOT NULL,COALESCE(v.identity_confirmed,0),COALESCE(v.provider_ok,0),
			COALESCE(v.exit_observed,0),v.exit_code,COALESCE(v.reply_id=r.id AND r.sender=a.alias AND r.recipient=q.sender,0)
			FROM requests q
			LEFT JOIN attempts a ON a.request_id=q.id AND a.session_key=q.session_key
			LEFT JOIN replies r ON r.reply_to=q.id AND r.attempt_token=a.token
			LEFT JOIN verifications v ON v.attempt_token=a.token
			WHERE q.session_key=? AND (?=0 OR q.rowid<?)
			ORDER BY q.rowid DESC LIMIT ?`, timelineTextLimit, timelineTextLimit, timelineTextLimit, timelineTextLimit, key, cursor, cursor, limit+1)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			if len(view.Items) == limit {
				view.HasMore = true
				break
			}
			var item TimelineItem
			var replyID, sender, recipient, text sql.NullString
			var exit sql.NullInt64
			var truncated, verified, identity, providerOK, exited, linked bool
			if err := rows.Scan(&item.ID, &item.Sender, &item.Text, &item.TextTruncated, &item.CreatedAt, &item.State, &item.RetryOf,
				&item.AttemptAlias, &item.AttemptState, &replyID, &sender, &recipient, &text, &truncated,
				&verified, &identity, &providerOK, &exited, &exit, &linked); err != nil {
				return err
			}
			if replyID.Valid {
				item.Reply = &TimelineReply{Reply: Reply{ID: replyID.String, ReplyTo: item.ID, Sender: sender.String, Recipient: recipient.String, Text: text.String}, TextTruncated: truncated}
			}
			item.Result = "not_verified"
			if exited && exit.Valid {
				code := int(exit.Int64)
				item.ExitCode = &code
			}
			if verified && exited && exit.Valid && exit.Int64 != 0 {
				item.Result = "process_failed"
			} else if verified && identity && providerOK && linked && item.Reply != nil {
				if exited && exit.Valid && exit.Int64 == 0 {
					item.Result = "verified_exit"
				} else if !exited && !exit.Valid && item.AttemptState == "turn_completed" && item.State == "completed" {
					item.Result = "verified_turn"
				}
			}
			view.Items = append(view.Items, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		// Reversing only this page lets the UI prepend older pages unchanged.
		for left, right := 0, len(view.Items)-1; left < right; left, right = left+1, right-1 {
			view.Items[left], view.Items[right] = view.Items[right], view.Items[left]
		}
		if view.HasMore {
			view.NextBefore = view.Items[0].ID
		}
		return nil
	})
	if err != nil {
		return Timeline{}, err
	}
	return view, nil
}
