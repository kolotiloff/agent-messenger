package localui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

// Every agent that can run tasks has its proper name in each of the window's
// name lists, so the window never shows an internal ID such as "cursor".
func TestWindowNamesEveryTaskAgent(t *testing.T) {
	raw, err := assets.ReadFile("assets/app.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(raw)
	for _, list := range []string{"names", "shortNames"} {
		m := regexp.MustCompile(`const ` + list + `=\{([^}]*)\}`).FindStringSubmatch(js)
		if m == nil {
			t.Fatalf("%s not found", list)
		}
		for provider := range core.TaskProviders {
			entry := regexp.MustCompile(provider + `:'([^']+)'`).FindStringSubmatch(m[1])
			if entry == nil {
				t.Errorf("%s has no name for %s", list, provider)
				continue
			}
			if !strings.HasPrefix(adapters.ProviderTitle(provider), entry[1]) || entry[1] == provider {
				t.Errorf("%s names %s %q", list, provider, entry[1])
			}
		}
	}
	html, err := assets.ReadFile("assets/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for provider := range core.TaskProviders {
		option := `<option value="` + provider + `" data-provider="` + provider + `">` + adapters.ProviderTitle(provider) + `</option>`
		if !strings.Contains(string(html), option) {
			t.Errorf("the agent filter has no %s", option)
		}
	}
}
