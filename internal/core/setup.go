package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var setupIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func ValidSetupID(id string) bool { return setupIDPattern.MatchString(id) }

// SetupNewClaude commits a new, fully configured session atomically. The exact
// client-generated UUID makes an ambiguous transport retry idempotent. Existing
// bindings are never reconfigured, even when their aliases or paths coincide.
func (s *Store) SetupNewClaude(environment, id, alias, cli, workspace string) error {
	if !setupIDPattern.MatchString(id) || strings.TrimSpace(alias) != alias || alias == "" || len(alias) > 128 || strings.ContainsAny(alias, "\r\n\x00") {
		return fmt.Errorf("%w: invalid new session identity", ErrRejected)
	}
	environment, err := CanonicalEnvironment(environment)
	if err != nil {
		return err
	}
	workspace, err = CanonicalEnvironment(workspace)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(cli) {
		return ErrRejected
	}
	cli, err = filepath.EvalSymlinks(cli)
	if err != nil {
		return ErrRejected
	}
	info, err := os.Stat(cli)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return ErrRejected
	}
	key, err := SessionKey("claude", environment, id)
	if err != nil {
		return err
	}
	return s.tx(func(tx *sql.Tx) error {
		var bound, oldCLI, oldWorkspace, mode string
		err := tx.QueryRow(`SELECT a.session_key,COALESCE(s.cli_path,''),COALESCE(s.workspace,''),s.start_mode FROM aliases a JOIN sessions s ON s.key=a.session_key WHERE a.alias=?`, alias).Scan(&bound, &oldCLI, &oldWorkspace, &mode)
		if err == nil {
			// Only repeat the identical operation; never modify a live/queued session.
			var setupCount int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM events WHERE kind='setup_new_claude' AND session_key=?`, key).Scan(&setupCount); err != nil {
				return err
			}
			if bound == key && oldCLI == cli && oldWorkspace == workspace && setupCount == 1 {
				return nil
			}
			return ErrRejected
		}
		if err != sql.ErrNoRows {
			return err
		}
		if reserved, err := aliasReservedBySetup(tx, alias); err != nil || reserved {
			if err == nil {
				err = ErrRejected
			}
			return err
		}
		var existing int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE key=?`, key).Scan(&existing); err != nil {
			return err
		}
		if existing != 0 {
			return ErrRejected
		}
		if _, err := tx.Exec(`INSERT INTO sessions(key,provider,environment,conversation_id,cli_path,workspace,start_mode) VALUES (?,'claude',?,?,?,?, 'new')`, key, environment, id, cli, workspace); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO aliases(alias,session_key) VALUES (?,?)`, alias, key); err != nil {
			return err
		}
		return insertEvent(tx, "setup_new_claude", key, "", "new isolated session configured; no message queued")
	})
}
