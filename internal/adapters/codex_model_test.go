package adapters

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCodexTurnModelRequiresContextFromThisRun(t *testing.T) {
	profile := t.TempDir()
	thread := "019a0000-0000-7000-8000-000000000001"
	created := time.Now().AddDate(0, 0, -3)
	launch := time.Now()
	dir := filepath.Join(profile, "sessions", filepath.FromSlash(created.Local().Format("2006/01/02")))
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-fixture-"+thread+".jsonl")
	line := func(at time.Time, model, effort string) string {
		return fmt.Sprintf(`{"timestamp":%q,"type":"turn_context","payload":{"model":%q,"effort":%q}}`+"\n", at.UTC().Format(time.RFC3339Nano), model, effort)
	}
	old := line(created, "gpt-6.2-sol", "low")
	write := func(contents string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(old)
	if got := CodexTurnModel(profile, thread, launch, created); got != "" {
		t.Fatalf("reused settings from the earlier turn: %q", got)
	}
	write(old + line(launch.Add(time.Second), "gpt-6.2-astra", "high") + `{"type":"turn_context"`)
	if got := CodexTurnModel(profile, thread, launch, created); got != "gpt-6.2-astra[effort=high]" {
		t.Fatalf("observed model = %q", got)
	}
	if got := CodexTurnModel(profile, "019a0000-0000-7000-8000-000000000002", launch, created); got != "" {
		t.Fatalf("used a different thread's model: %q", got)
	}
}
