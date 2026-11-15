package mcp

import (
	"errors"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

// T-158: list_agents names the newest downloaded release with its notes and
// marks computers that run an older version; without a release nothing is added.
func TestListAgentsNewestRelease(t *testing.T) {
	c := &coordinator{release: func() (updatefeed.Manifest, error) {
		return updatefeed.Manifest{Build: 75, Version: "0.0.0-preview.75", Notes: " Fixes the notices. " + strings.Repeat("x", releaseNotesMax)}, nil
	}}
	out := map[string]any{"computers": []core.ComputerSummary{
		{Name: "old", Version: "0.0.0-preview.72"},
		{Name: "same", Version: "0.0.0-preview.75"},
		{Name: "unknown"},
		{Name: "dev", Version: "dev"},
	}}
	c.addNewestRelease(out)
	newest, _ := out["newest_release"].(map[string]any)
	if newest["version"] != "0.75" || newest["full_version"] != "0.0.0-preview.75" {
		t.Fatalf("newest %+v", newest)
	}
	notes, _ := newest["notes"].(string)
	if !strings.HasPrefix(notes, "Fixes the notices.") || !strings.HasSuffix(notes, "…") || len([]rune(notes)) != releaseNotesMax+1 {
		t.Fatalf("notes %q", notes[:40])
	}
	list := out["computers"].([]core.ComputerSummary)
	for i, want := range []bool{true, false, false, false} {
		if list[i].UpdateAvailable != want {
			t.Fatalf("%s update_available=%v", list[i].Name, list[i].UpdateAvailable)
		}
	}

	none := map[string]any{"computers": []core.ComputerSummary{{Version: "0.0.0-preview.1"}}}
	(&coordinator{release: func() (updatefeed.Manifest, error) { return updatefeed.Manifest{}, errors.New("no release") }}).addNewestRelease(none)
	if _, ok := none["newest_release"]; ok || none["computers"].([]core.ComputerSummary)[0].UpdateAvailable {
		t.Fatalf("added without a release: %+v", none)
	}
	(&coordinator{}).addNewestRelease(none) // no data directory: nothing read
	if _, ok := none["newest_release"]; ok {
		t.Fatal("read without a data directory")
	}
}
