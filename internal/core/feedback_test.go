package core

import (
	"archive/zip"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// T-135: remarks of the owner and the coordinator stay on this computer and
// leave it only as one exported file with their pictures.
func TestFeedbackJournalAndExport(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	dir := t.TempDir()
	if _, _, err := st.ExportFeedback(dir, time.Now()); !errors.Is(err, ErrRejected) {
		t.Fatalf("empty export %v", err)
	}
	png := []byte("\x89PNG\r\n\x1a\nfake")
	if _, err := st.AddFeedback(AudienceOwner, "Game", "Кнопка «Закрыть» не видна", "0.61", "image/png", png); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddFeedback(AudienceCoordinator, "Game", "wait returned an event twice", "0.61", "", nil); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct{ author, text, kind string }{{"worker", "x", ""}, {AudienceOwner, " ", ""}, {AudienceOwner, "x", "image/gif"}} {
		var img []byte
		if bad.kind != "" {
			img = png
		}
		if _, err := st.AddFeedback(bad.author, "", bad.text, "", bad.kind, img); !errors.Is(err, ErrRejected) {
			t.Fatalf("bad remark %+v accepted: %v", bad, err)
		}
	}
	if n, _ := st.FeedbackCount(); n != 2 {
		t.Fatalf("count %d", n)
	}
	path, n, err := st.ExportFeedback(dir, time.Now())
	if err != nil || n != 2 || filepath.Dir(path) != dir {
		t.Fatalf("export %s %d %v", path, n, err)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var md string
	images := 0
	for _, f := range z.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		if f.Name == "feedback.md" {
			md = string(b)
		} else if strings.HasPrefix(f.Name, "images/") && string(b) == string(png) {
			images++
		}
	}
	if images != 1 || !strings.Contains(md, "Кнопка «Закрыть» не видна") || !strings.Contains(md, "coordinator") || !strings.Contains(md, "![picture](images/") {
		t.Fatalf("zip images %d md %q", images, md)
	}
}
