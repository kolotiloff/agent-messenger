package mcp

import (
	"encoding/json"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/core"
)

// The report tool takes optional fields and stores one labelled text (T-050).
func TestReportArguments(t *testing.T) {
	var args map[string]any
	_ = json.Unmarshal([]byte(`{"text":"Готово.","changes":"cs:42","tests":"12/12","files":["Docs/Audits/a.md"]}`), &args)
	got, err := reportText(args, &core.ReportParts{})
	if err != nil || got != "Готово.\nИзменения: cs:42\nТесты: 12/12\nФайлы: Docs/Audits/a.md" {
		t.Fatalf("%q %v", got, err)
	}
	if got, err := reportText(map[string]any{"text": "Готово."}, &core.ReportParts{}); err != nil || got != "Готово." {
		t.Fatalf("plain %q %v", got, err)
	}
	for _, bad := range []string{`{"text":"x","files":"a.md"}`, `{"text":"x","files":[1]}`, `{"text":"x","extra":"y"}`, `{"changes":"cs:1"}`, `{"text":"x","tests":3}`} {
		var a map[string]any
		_ = json.Unmarshal([]byte(bad), &a)
		if _, err := reportText(a, &core.ReportParts{}); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	// The listed tool says so.
	for _, tool := range taskTools() {
		if tool["name"] == "report" {
			props := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
			for _, key := range []string{"text", "changes", "tests", "files"} {
				if props[key] == nil {
					t.Fatalf("no %s in the report schema", key)
				}
			}
		}
	}
}
