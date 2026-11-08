package core

import (
	"database/sql"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Resource locks (T-072, D-086). A task may name a resource, such as
// «Unity editor», that only one worker on a computer may use at a time. A
// task with a lock does not launch while an earlier task with the same lock
// is open (not done, failed or cancelled) on the same computer. A blocked
// task keeps its lock: whether it still uses the resource is unknown. The
// lock travels with a task sent to another computer, which applies it
// there; a computer before it ignores the lock.

// MaxTaskLock bounds a lock name.
const MaxTaskLock = 40

// lockHolderQuery selects the open tasks that hold t's lock: same lock,
// same computer, created earlier. It is used inside a query over tasks t.
const lockHolderQuery = `SELECT o.id FROM tasks o WHERE o.resource=t.resource AND o.computer=t.computer AND o.id<>t.id
 AND o.state NOT IN ('done','failed','cancelled')
 AND (o.created_at<t.created_at OR (o.created_at=t.created_at AND o.rowid<t.rowid))`

// normalizeLock trims and lowercases a lock name, so «Unity editor» and
// «unity EDITOR» are one lock. It reports false for a name that is too long
// or not one plain line.
func normalizeLock(s string) (string, bool) {
	if !utf8.ValidString(s) {
		return "", false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", false
		}
	}
	s = strings.ToLower(strings.Join(strings.Fields(s), " "))
	return s, utf8.RuneCountInString(s) <= MaxTaskLock
}

// TaskLockHolder returns the task that keeps this task from launching, or
// "" when none does.
func (s *Store) TaskLockHolder(id string) (string, error) {
	var holder sql.NullString
	err := s.db.QueryRow(`SELECT (`+lockHolderQuery+` ORDER BY o.created_at,o.rowid LIMIT 1) FROM tasks t WHERE t.id=? AND t.resource<>''`, id).Scan(&holder)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return holder.String, err
}

// waitingLock returns the lock a task waits for, or "" when it waits for
// none.
func (s *Store) waitingLock(id string) (string, error) {
	holder, err := s.TaskLockHolder(id)
	if err != nil || holder == "" {
		return "", err
	}
	var lock string
	err = s.db.QueryRow(`SELECT resource FROM tasks WHERE id=?`, id).Scan(&lock)
	return lock, err
}
