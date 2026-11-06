package core

import "database/sql"

// The journal records an explicit action; the actual permission is only in the
// live supervisor. A crash between this commit and the grant cannot enable a CLI.
func (s *Store) AuthorizeSetup(alias string) error {
	return s.tx(func(tx *sql.Tx) error {
		var key string
		err := tx.QueryRow(`SELECT s.key FROM sessions s JOIN aliases a ON a.session_key=s.key
            WHERE a.alias=? AND s.provider='claude' AND s.start_mode='new'
            AND s.active_token IS NULL AND s.decision IS NULL
            AND COALESCE(s.cli_path,'')!='' AND COALESCE(s.workspace,'')!=''
            AND (SELECT COUNT(*) FROM events e WHERE e.session_key=s.key AND e.kind='setup_new_claude')=1`, alias).Scan(&key)
		if err == sql.ErrNoRows {
			return ErrRejected
		}
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE control SET paused=0 WHERE id=1`); err != nil {
			return err
		}
		return insertEvent(tx, "authorize_setup", key, "", "explicit permission for this test session; not restored after restart")
	})
}
