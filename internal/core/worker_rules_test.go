package core

import (
	"errors"
	"strings"
	"testing"
)

// The owner writes rules for the project's workers once; every task started
// afterwards carries them, and the worker reads them with the task (T-050).
func TestProjectWorkerRules(t *testing.T) {
	f := newTaskFixture(t)
	if rules, err := f.st.ProjectWorkerRules("Game"); err != nil || rules != "" {
		t.Fatalf("default %q %v", rules, err)
	}
	before := f.start(t, "op-before", "codex", "Fix the menu")
	if err := f.st.SetProjectWorkerRules("Game", "  Отчёт до 8 строк.\nПодробности — в Docs/Audits/.  "); err != nil {
		t.Fatal(err)
	}
	if rules, _ := f.st.ProjectWorkerRules("Game"); rules != "Отчёт до 8 строк.\nПодробности — в Docs/Audits/." {
		t.Fatalf("stored %q", rules)
	}
	for _, bad := range []string{strings.Repeat("я", MaxWorkerRules+1), "a\x00b"} {
		if err := f.st.SetProjectWorkerRules("Game", bad); !errors.Is(err, ErrRejected) {
			t.Fatalf("accepted %q: %v", bad, err)
		}
	}
	if err := f.st.SetProjectWorkerRules("Nope", "x"); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown project: %v", err)
	}
	after := f.start(t, "op-after", "codex", "Fix the map")

	r := f.launch(t, after, "019a0000-0000-7000-8000-0000000000a1")
	brief, err := f.st.ReadTask(r.ID, r.Receipt, false)
	if err != nil || brief.Rules != "Отчёт до 8 строк.\nПодробности — в Docs/Audits/." {
		t.Fatalf("brief %+v %v", brief, err)
	}
	// A task keeps the rules it was started with.
	if err := f.st.SetProjectWorkerRules("Game", ""); err != nil {
		t.Fatal(err)
	}
	if again, _ := f.st.ReadTask(r.ID, r.Receipt, false); again.Rules != brief.Rules {
		t.Fatalf("changed rules reached a started task: %q", again.Rules)
	}
	r0 := f.launch(t, before, "019a0000-0000-7000-8000-0000000000a2")
	if old, _ := f.st.ReadTask(r0.ID, r0.Receipt, false); old.Rules != "" {
		t.Fatalf("an earlier task got rules: %q", old.Rules)
	}
}

// Rules travel with a task to the computer that runs it.
func TestWorkerRulesReachTheOtherComputer(t *testing.T) {
	p := newNetPair(t)
	if err := p.a.st.SetProjectWorkerRules("Game", "Короткий отчёт."); err != nil {
		t.Fatal(err)
	}
	// The other computer's own rules are not used for a task it was sent.
	if err := p.b.st.SetProjectWorkerRules("Game", "Чужие правила."); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op1", "codex", "Build level 1")
	p.carry(t, true)
	m := p.mirror(t, id)
	r := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000b1")
	brief, err := p.b.st.ReadTask(r.ID, r.Receipt, false)
	if err != nil || brief.Rules != "Короткий отчёт." {
		t.Fatalf("brief %+v %v", brief, err)
	}
}

// A structured report becomes one text with labelled lines, so it travels
// and shows like any report (T-050).
func TestStructuredReport(t *testing.T) {
	for _, c := range []struct {
		in   ReportParts
		want string
	}{
		{ReportParts{Text: "Готово."}, "Готово."},
		{ReportParts{Text: "Готово.", Changes: "cs:1234", Tests: "EditMode 12/12", Files: []string{"Docs/Audits/a.md", " Assets/b.cs "}},
			"Готово.\nИзменения: cs:1234\nТесты: EditMode 12/12\nФайлы: Docs/Audits/a.md, Assets/b.cs"},
	} {
		got, err := c.in.Compose()
		if err != nil || got != c.want {
			t.Fatalf("%+v: %q %v", c.in, got, err)
		}
	}
	for _, bad := range []ReportParts{
		{},
		{Text: "x", Changes: "a\nb"},
		{Text: "x", Files: make([]string, MaxReportFiles+1)},
		{Text: "x", Files: []string{""}},
		{Text: "x", Tests: strings.Repeat("я", 501)},
	} {
		if _, err := bad.Compose(); !errors.Is(err, ErrRejected) {
			t.Fatalf("accepted %+v", bad)
		}
	}
}
