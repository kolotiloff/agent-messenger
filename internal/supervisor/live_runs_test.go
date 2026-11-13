package supervisor

import (
	"testing"
	"time"
)

// T-149: a started worker is listed with the age of its last output until
// it exits; two runs of one task give the freshest.
func TestLiveWorkers(t *testing.T) {
	var runs taskRuns
	a := runs.started("run-1", "task-a")
	a.lastOutput.Store(time.Now().Add(-90 * time.Second).UnixNano())
	runs.started("run-2", "task-b")
	old := runs.started("run-3", "task-b")
	old.lastOutput.Store(time.Now().Add(-time.Hour).UnixNano())
	got := runs.workers()
	if len(got) != 2 || got["task-a"] < 89 || got["task-a"] > 95 || got["task-b"] > 2 {
		t.Fatalf("workers %v", got)
	}
	runs.exited("run-1")
	if _, ok := runs.workers()["task-a"]; ok {
		t.Fatal("exited worker still listed")
	}
}
