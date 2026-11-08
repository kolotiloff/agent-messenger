package core

import (
	"os"
	"strings"
	"testing"
)

// The owner names this computer for the network; empty goes back to the
// system's name.
func TestComputerName(t *testing.T) {
	f := newTaskFixture(t)
	system, _ := os.Hostname()
	if name, err := f.st.ComputerName(); err != nil || name != system {
		t.Fatalf("default %q %v", name, err)
	}
	if err := f.st.SetComputerName("  Студия  "); err != nil {
		t.Fatal(err)
	}
	if name, _ := f.st.ComputerName(); name != "Студия" {
		t.Fatalf("set %q", name)
	}
	if v, err := f.st.WindowView("", ""); err != nil || v.Name != "Студия" {
		t.Fatalf("window %q %v", v.Name, err)
	}
	for _, bad := range []string{strings.Repeat("я", MaxComputerName+1), "a\nb", "a\x00b"} {
		if err := f.st.SetComputerName(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if err := f.st.SetComputerName(""); err != nil {
		t.Fatal(err)
	}
	if name, _ := f.st.ComputerName(); name != system {
		t.Fatalf("reset %q", name)
	}
}
