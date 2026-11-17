package localui

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// T-196: the window opens on the port it had last time, so the page keeps
// what it stored (theme, language); a taken port gives a new one.
func TestWindowKeepsItsPort(t *testing.T) {
	root := t.TempDir()
	port := func(ln net.Listener) int { return ln.Addr().(*net.TCPAddr).Port }
	first, err := listenWindow(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	p := port(first)
	first.Close()
	again, err := listenWindow(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if port(again) != p {
		t.Fatalf("port %d, then %d", p, port(again))
	}
	// Still held: another port is used and remembered.
	other, err := listenWindow(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	again.Close()
	if port(other) == p {
		t.Fatal("a taken port was used twice")
	}
	raw, _ := os.ReadFile(filepath.Join(root, windowPortFile))
	if string(raw) != strconv.Itoa(port(other))+"\n" {
		t.Fatalf("remembered %q", raw)
	}
	if err := os.WriteFile(filepath.Join(root, windowPortFile), []byte("junk"), 0o600); err != nil {
		t.Fatal(err)
	}
	ln, err := listenWindow(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	ln.Close()
}
