package modelcatalog

import (
	_ "embed"
	"encoding/json"
)

// catalog.json is a snapshot of the Codex app-server model/list and Cursor
// CLI --list-models catalogs, and the Claude Code model configuration.
// Effort values are limited to those understood by Messenger.
var (
	//go:embed catalog.json
	snapshot   []byte
	selections = loadSelections(snapshot)
)

func loadSelections(raw []byte) map[string][]Option {
	var byProvider map[string][]Option
	if err := json.Unmarshal(raw, &byProvider); err != nil {
		panic("invalid embedded model catalog: " + err.Error())
	}
	return byProvider
}

// Selections combines the starting choices with the CLI catalog. Availability
// for a particular account may differ.
func Selections(provider string) []Option {
	out := append([]Option(nil), catalog[provider]...)
	seen := make(map[string]bool, len(out))
	for _, option := range out {
		seen[option.Model] = true
	}
	for _, option := range selections[provider] {
		if _, _, err := Resolve(provider, option.Model); err == nil && !seen[option.Model] {
			out = append(out, option)
			seen[option.Model] = true
		}
	}
	return out
}
