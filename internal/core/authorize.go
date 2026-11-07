package core

import "database/sql"

// The journal records an explicit action; the actual permission is only in the
// live supervisor. A crash between this commit and the grant cannot enable a CLI.
func (s *Store) AuthorizeSetup(alias string) error {
	return s.tx(func(tx *sql.Tx) error {
		key, err := authorizableSetup(tx, alias)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE control SET paused=0 WHERE id=1`); err != nil {
			return err
		}
		return insertEvent(tx, "authorize_setup", key, "", "explicit permission for this test session; not restored after restart")
	})
}

// CheckAuthorizeSetup reports, without changing anything, whether alias is a
// new test session the window may authorize. The supervisor asks it before it
// grants model calls, so a refused alias is never allowed, not even briefly.
func (s *Store) CheckAuthorizeSetup(alias string) error {
	return s.tx(func(tx *sql.Tx) error {
		_, err := authorizableSetup(tx, alias)
		return err
	})
}

func authorizableSetup(tx *sql.Tx, alias string) (string, error) {
	var key string
	err := tx.QueryRow(`SELECT s.key FROM sessions s JOIN aliases a ON a.session_key=s.key
            WHERE a.alias=? AND s.provider='claude' AND s.start_mode='new'
            AND s.active_token IS NULL AND s.decision IS NULL
            AND COALESCE(s.cli_path,'')!='' AND COALESCE(s.workspace,'')!=''
            AND (SELECT COUNT(*) FROM events e WHERE e.session_key=s.key AND e.kind='setup_new_claude')=1`, alias).Scan(&key)
	if err == sql.ErrNoRows {
		return "", ErrRejected
	}
	return key, err
}

// AliasInUse reports whether a session already answers to alias.
func (s *Store) AliasInUse(alias string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM aliases WHERE alias=?`, alias).Scan(&n)
	return n > 0, err
}
