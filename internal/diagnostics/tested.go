package diagnostics

import (
	"strconv"
	"strings"
)

// TestedCLI is, per provider, the CLI version this release was checked
// against (T-139). The daily CLI watch (.github/workflows/cli-watch.yml)
// reports a newer version; after checking it, raise the number here and
// in the next release. An empty value means no version is recorded and no
// mark is shown.
var TestedCLI = map[string]string{
	"claude": "2.1.322",
	"codex":  "0.167.2",
	"cursor": "",
}

// Tested compares an installed CLI version with the checked one: "tested"
// (same), "newer", "older", or "" when either is unknown.
func Tested(provider, version string) string {
	want := TestedCLI[provider]
	if want == "" || version == "" {
		return ""
	}
	switch c := compareVersions(version, want); {
	case c > 0:
		return "newer"
	case c < 0:
		return "older"
	}
	return "tested"
}

// compareVersions compares dotted versions part by part ("2.1.322",
// "0.167.2", "2026.11.05-86047df"); a part that is not a number ends the
// comparison.
func compareVersions(a, b string) int {
	pa, pb := versionParts(a), versionParts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	var out []int
	for _, p := range strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '-' || r == ' ' }) {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}
