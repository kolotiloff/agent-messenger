package core

import (
	"database/sql"
	"fmt"
)

// Automatic updates (T-277, D-191). The owner turns them on in the program
// settings of each computer; nothing on the network can change the setting.
// The client then queues an update of this computer, as if the owner had
// asked for one, once for each newer release: a release whose update was
// already queued is never queued again on its own, whatever its result.

// AutoUpdate reports whether this computer updates itself. Off until the
// owner turns it on.
func (s *Store) AutoUpdate() (bool, error) {
	var on bool
	err := s.tx(func(tx *sql.Tx) error {
		v, err := metaValue(tx, "auto_update")
		on = v == "1"
		return err
	})
	return on, err
}

// SetAutoUpdate turns automatic updates on or off.
func (s *Store) SetAutoUpdate(on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	return s.tx(func(tx *sql.Tx) error { return setMeta(tx, "auto_update", v) })
}

// QueueAutoUpdate queues an update of this computer for release version,
// unless that release was queued on its own before or an update of this
// computer is already queued. It reports whether it queued one. The
// release is remembered in the same transaction, so a failed update is not
// started again for it.
func (s *Store) QueueAutoUpdate(version string) (bool, error) {
	if version == "" || len(version) > maxUpdateVersion {
		return false, fmt.Errorf("%w: unknown release", ErrRejected)
	}
	queued := false
	err := s.tx(func(tx *sql.Tx) error {
		on, err := metaValue(tx, "auto_update")
		if err != nil || on != "1" {
			return err
		}
		tried, err := metaValue(tx, "auto_update_release")
		if err != nil || tried == version {
			return err
		}
		var pending int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM pending_updates`).Scan(&pending); err != nil {
			return err
		}
		if pending > 0 {
			return nil
		}
		if err := setMeta(tx, "auto_update_release", version); err != nil {
			return err
		}
		if _, err := s.queueLocalUpdate(tx, ""); err != nil {
			return err
		}
		queued = true
		return nil
	})
	return queued, err
}
