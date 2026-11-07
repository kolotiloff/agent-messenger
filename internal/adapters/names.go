package adapters

// ProviderTitle is the name shown to people for an agent ID: "Claude Code",
// "Codex", "Cursor". An unknown ID is returned as it is.
func ProviderTitle(provider string) string {
	switch provider {
	case "claude":
		return "Claude Code"
	case "codex":
		return "Codex"
	case "cursor":
		return "Cursor"
	}
	return provider
}
