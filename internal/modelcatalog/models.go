// Package modelcatalog defines explicit worker selections, not account entitlements.
package modelcatalog

import (
	"fmt"
	"regexp"
)

var plain = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/\[\]-]{0,79}$`)
var selection = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._:/-]*)\[effort=(low|medium|high|xhigh|max)\]$`)

// Split accepts the legacy model syntax or one explicit Messenger effort suffix.
// The suffix is translated to CLI arguments, never passed as a Codex/Claude ID.
func Split(value string) (model, effort string, err error) {
	if value == "" || plain.MatchString(value) {
		return value, "", nil
	}
	if len(value) <= 80 {
		if m := selection.FindStringSubmatch(value); m != nil {
			return m[1], m[2], nil
		}
	}
	return "", "", fmt.Errorf("invalid model selection %q", value)
}

func Resolve(provider, value string) (model, effort string, err error) {
	model, effort, err = Split(value)
	if err == nil && effort != "" && provider != "codex" && provider != "claude" {
		err = fmt.Errorf("%s needs its CLI model ID including the effort level", provider)
	}
	return
}

// Option gives coordinators the exact start_task value and intended task scope.
// Purpose is guidance, not a measured benchmark or a claim of account access.
type Option struct {
	Model   string `json:"model"`
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
}

var catalog = map[string][]Option{
	"codex": {
		{"gpt-6-luna[effort=high]", "GPT-6 Luna High", "Small, focused edits and routine checks"},
		{"gpt-6.1-sol[effort=medium]", "GPT-6.1 Sol Medium", "Everyday implementation and tests"},
		{"gpt-6-astra[effort=medium]", "GPT-6 Astra Medium", "Complex features, debugging and refactoring"},
		{"gpt-6-astra[effort=high]", "GPT-6 Astra High", "Hard architecture and cross-system investigation"},
	},
	"claude": {
		{"claude-haiku-5-5[effort=low]", "Claude Haiku 5.5 Low", "Small, mechanical edits"},
		{"claude-sonnet-5-5[effort=medium]", "Claude Sonnet 5.5 Medium", "Everyday implementation and tests"},
		{"claude-opus-5-5[effort=medium]", "Claude Opus 5.5 Medium", "Complex features, debugging and refactoring"},
		{"claude-fable-5-2[effort=high]", "Claude Fable 5.2 High", "Hard architecture and long investigations; requires account access"},
	},
	"cursor": {
		{"composer-2.5", "Composer 2.5", "Quick, focused coding; CLI default effort"},
		{"cursor-grok-4.6-medium", "Grok 4.6 Medium", "Everyday implementation and tests"},
		{"grok-4.7-medium", "Grok 4.7 Medium", "Complex features, debugging and refactoring"},
		{"grok-4.7-high", "Grok 4.7 High", "Hard investigations; higher effort requires a supporting plan"},
	},
}

func Defaults(provider string) []string {
	out := []string{}
	for _, o := range catalog[provider] {
		out = append(out, o.Model)
	}
	return out
}

func Describe(provider string, listed []string) []Option {
	var out []Option
	for _, model := range listed {
		for _, o := range catalog[provider] {
			if o.Model == model {
				out = append(out, o)
				break
			}
		}
	}
	return out
}
