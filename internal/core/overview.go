package core

import "database/sql"

const overviewLimit = 100

// Overview is a bounded projection of recorded local state, not a readiness
// probe. It deliberately omits CLI IDs, paths, tokens and message contents.
type Overview struct {
	SchemaVersion int           `json:"schema_version"`
	Paused        bool          `json:"paused"`
	Sessions      []SessionCard `json:"sessions"`
	Truncated     bool          `json:"truncated"`
}

type SessionCard struct {
	CanLaunchTest        bool     `json:"can_launch_test,omitempty"`
	Provider             string   `json:"provider"`
	Aliases              []string `json:"aliases"`
	AliasesTruncated     bool     `json:"aliases_truncated"`
	LaunchConfigured     bool     `json:"launch_configured"`
	Authentication       string   `json:"authentication"`
	Readiness            string   `json:"readiness"`
	OwnerRetained        bool     `json:"owner_retained"`
	RecordedAttemptState string   `json:"recorded_attempt_state"`
	RecordedStreamState  string   `json:"recorded_stream_state"`
	DecisionPending      bool     `json:"decision_pending"`
	Queued               int      `json:"queued"`
	Uncertain            int      `json:"uncertain"`
}

func (s *Store) Overview() (Overview, error) {
	view := Overview{SchemaVersion: 1, Sessions: []SessionCard{}}
	err := s.tx(func(tx *sql.Tx) error {
		var err error
		view.Paused, err = isPaused(tx)
		if err != nil {
			return err
		}
		rows, err := tx.Query(`SELECT s.key, s.provider,
			COALESCE(s.cli_path,'')<>'' AND COALESCE(s.workspace,'')<>'',
			s.active_token IS NOT NULL, COALESCE(a.state,'none'),
			COALESCE(r.state,'none'), s.decision IS NOT NULL,
			(SELECT COUNT(*) FROM requests q WHERE q.session_key=s.key AND q.state='queued'),
			(SELECT COUNT(*) FROM requests q WHERE q.session_key=s.key AND q.state='uncertain'),
            s.provider='claude' AND s.start_mode='new' AND s.active_token IS NULL AND s.decision IS NULL
            AND COALESCE(s.cli_path,'')!='' AND COALESCE(s.workspace,'')!=''
            AND (SELECT COUNT(*) FROM events e WHERE e.session_key=s.key AND e.kind='setup_new_claude')=1
			FROM sessions s LEFT JOIN attempts a ON a.token=s.active_token
			LEFT JOIN stream_runs r ON r.session_key=s.key
			ORDER BY s.rowid LIMIT ?`, overviewLimit+1)
		if err != nil {
			return err
		}
		var keys []string
		for rows.Next() {
			var key string
			card := SessionCard{Aliases: []string{}, Authentication: "not_checked", Readiness: "not_checked"}
			if err := rows.Scan(&key, &card.Provider, &card.LaunchConfigured, &card.OwnerRetained,
				&card.RecordedAttemptState, &card.RecordedStreamState, &card.DecisionPending,
				&card.Queued, &card.Uncertain, &card.CanLaunchTest); err != nil {
				rows.Close()
				return err
			}
			if len(view.Sessions) == overviewLimit {
				view.Truncated = true
				break
			}
			keys = append(keys, key)
			view.Sessions = append(view.Sessions, card)
		}
		readErr := rows.Err()
		closeErr := rows.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		for i, key := range keys {
			rows, err := tx.Query(`SELECT alias FROM aliases WHERE session_key=? ORDER BY alias LIMIT ?`, key, overviewLimit+1)
			if err != nil {
				return err
			}
			for rows.Next() {
				var alias string
				if err := rows.Scan(&alias); err != nil {
					rows.Close()
					return err
				}
				if len(view.Sessions[i].Aliases) == overviewLimit {
					view.Sessions[i].AliasesTruncated = true
					break
				}
				view.Sessions[i].Aliases = append(view.Sessions[i].Aliases, alias)
			}
			readErr, closeErr = rows.Err(), rows.Close()
			if readErr != nil {
				return readErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return nil
	})
	if err != nil {
		return Overview{}, err
	}
	return view, nil
}
