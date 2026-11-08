package localui

import (
	"regexp"
	"strings"
	"testing"
)

// Every Russian text of the window has an English translation (T-059): each
// tr('…') of app.js and each text and readable attribute of index.html is a
// key of the dictionary in i18n.js, and app.js has no Russian text that
// skips tr().
func TestWindowTextsTranslated(t *testing.T) {
	read := func(name string) string {
		raw, err := assets.ReadFile("assets/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	dict := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s*'((?:[^'\\\n]|\\.)*)':'`).FindAllStringSubmatch(read("i18n.js"), -1) {
		dict[m[1]] = true
	}
	if len(dict) < 300 {
		t.Fatalf("dictionary has %d entries", len(dict))
	}
	cyrillic := regexp.MustCompile(`[А-Яа-яЁё]`)
	js := read("app.js")
	for _, m := range regexp.MustCompile(`tr\('((?:[^'\\\n]|\\.)*)'\)`).FindAllStringSubmatch(js, -1) {
		if !dict[m[1]] {
			t.Errorf("app.js: no translation for %q", m[1])
		}
	}
	// Russian outside tr() and comments: only the plural helpers and the
	// letter used to measure the font.
	literal := regexp.MustCompile(`'((?:[^'\\\n]|\\.)*)'`)
	for i, line := range strings.Split(js, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") || strings.Contains(line, "function tasksWord") || strings.Contains(line, "function tokensWord") {
			continue
		}
		bare := regexp.MustCompile(`tr\('(?:[^'\\\n]|\\.)*'\)`).ReplaceAllString(line, "")
		for _, m := range literal.FindAllStringSubmatch(bare, -1) {
			if cyrillic.MatchString(m[1]) && m[1] != "Н" {
				t.Errorf("app.js line %d: %q without tr()", i+1, m[1])
			}
		}
	}
	html := read("index.html")
	var texts []string
	for _, m := range regexp.MustCompile(`>([^<>]+)<`).FindAllStringSubmatch(html, -1) {
		texts = append(texts, strings.TrimSpace(m[1]))
	}
	for _, m := range regexp.MustCompile(`(?:aria-label|title|placeholder)="([^"]+)"`).FindAllStringSubmatch(html, -1) {
		texts = append(texts, m[1])
	}
	for _, text := range texts {
		if cyrillic.MatchString(text) && text != "Русский" && !dict[text] {
			t.Errorf("index.html: no translation for %q", text)
		}
	}
}
