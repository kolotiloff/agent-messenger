package diagnostics

import "testing"

func TestTestedVersions(t *testing.T) {
	old := TestedCLI
	t.Cleanup(func() { TestedCLI = old })
	TestedCLI = map[string]string{"claude": "2.1.322", "codex": "0.167.2", "cursor": "2026.11.05-86047df", "none": ""}
	for _, c := range []struct{ provider, version, want string }{
		{"claude", "2.1.322", "tested"},
		{"claude", "2.1.323", "newer"},
		{"claude", "2.2.0", "newer"},
		{"claude", "2.1.40", "older"},
		{"codex", "0.167.2", "tested"},
		{"codex", "0.170.0", "newer"},
		{"cursor", "2026.11.12-aaaaaaa", "newer"},
		{"cursor", "2026.11.05-86047df", "tested"},
		{"none", "1.0.0", ""},
		{"claude", "", ""},
	} {
		if got := Tested(c.provider, c.version); got != c.want {
			t.Errorf("%s %s: %q, want %q", c.provider, c.version, got, c.want)
		}
	}
}
