package core

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// workPlan returns the query plan of counting work under where.
func workPlan(t *testing.T, db interface {
	Query(string, ...any) (*sql.Rows, error)
}, where string) string {
	t.Helper()
	rows, err := db.Query(`EXPLAIN QUERY PLAN SELECT COUNT(*) FROM tasks t WHERE ` + where)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return strings.Join(plan, "; ")
}

// T-272: counting the work an update waits for reads only the tasks and
// runs in a current state, never every task a project ever had.
func checkWorkPlan(t *testing.T, name string, db interface {
	Query(string, ...any) (*sql.Rows, error)
}) {
	t.Helper()
	for _, where := range []string{liveWorkWhere, activeWorkWhere} {
		plan := workPlan(t, db, where)
		if strings.Contains(plan, "SCAN") || !strings.Contains(plan, "tasks_state") || !strings.Contains(plan, "task_runs_state") {
			t.Fatalf("%s reads more than current work: %s", name, plan)
		}
	}
}

func TestWorkCountUsesStateIndexes(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	checkWorkPlan(t, "computer database", st.db)

	f := newSplitRunFixture(t)
	s, err := openSplitStore(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	files, err := splitProjectFiles(context.Background(), s.db)
	if err != nil || len(files) == 0 {
		t.Fatalf("project files: %v %v", files, err)
	}
	for _, file := range files {
		db, err := s.split.open(file)
		if err != nil {
			t.Fatal(err)
		}
		checkWorkPlan(t, file, db)
	}
	// A project database made before T-272 gets the indexes when opened.
	db, _ := s.split.open(files[0])
	if _, err := db.Exec(`DROP INDEX tasks_state; DROP INDEX task_runs_state`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = openSplitStore(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if db, err = s.split.open(files[0]); err != nil {
		t.Fatal(err)
	}
	checkWorkPlan(t, "older "+files[0], db)
}
