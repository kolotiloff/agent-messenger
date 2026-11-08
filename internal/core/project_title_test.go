package core

import (
	"errors"
	"testing"
)

// The owner can rename a project in the window (T-065). Only the name shown
// changes: tasks, the coordinator chat and other computers keep using the
// name the project was created with, so nothing already linked to it breaks.
func TestProjectTitle(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "codex", "Fix the menu")
	title := func() (string, string) {
		view, err := f.st.WindowView("Game", "2000-01-01")
		if err != nil {
			t.Fatal(err)
		}
		shown := ""
		for _, p := range view.Projects {
			if p.Name == "Game" {
				shown = p.Title
			}
		}
		return view.ProjectTitle, shown
	}
	if v, p := title(); v != "Game" || p != "Game" {
		t.Fatalf("title before a rename: view %q, list %q", v, p)
	}
	if err := f.st.SetProjectTitle("Game", "  Гонки 2  "); err != nil {
		t.Fatal(err)
	}
	if v, p := title(); v != "Гонки 2" || p != "Гонки 2" {
		t.Fatalf("title after a rename: view %q, list %q", v, p)
	}
	view, err := f.st.WindowView("Game", "2000-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if view.Project != "Game" || len(view.Tasks) != 1 || view.Tasks[0].ID != id {
		t.Fatalf("the project's key or its tasks changed: %q %+v", view.Project, view.Tasks)
	}
	// Another project may not take the same shown name, nor a name equal to
	// another project's key.
	if err := f.st.CreateProject("Other", realDir(t, t.TempDir())); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"Гонки 2", "гонки 2", "Game", "", "a\nb", "x\x00"} {
		if err := f.st.SetProjectTitle("Other", bad); !errors.Is(err, ErrRejected) {
			t.Fatalf("title %q accepted: %v", bad, err)
		}
	}
	// A new project may not take the name another project shows.
	if err := f.st.CreateProject("Race", realDir(t, t.TempDir())); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetProjectTitle("Race", "Ралли"); err != nil {
		t.Fatal(err)
	}
	if err := f.st.CreateProject("Ралли", realDir(t, t.TempDir())); !errors.Is(err, ErrRejected) {
		t.Fatalf("a project took another project's shown name: %v", err)
	}
	if err := f.st.SetProjectTitle("Missing", "Новое"); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown project renamed: %v", err)
	}
	// Going back to the original name is allowed.
	if err := f.st.SetProjectTitle("Game", "Game"); err != nil {
		t.Fatal(err)
	}
	if v, _ := title(); v != "Game" {
		t.Fatalf("title after going back %q", v)
	}
}
