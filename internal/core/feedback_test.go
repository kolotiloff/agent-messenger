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
	if _, _, _, err := st.ExportFeedback(dir, time.Now()); !errors.Is(err, ErrRejected) {
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
	if n, fresh, _ := st.FeedbackCount(); n != 2 || fresh != 2 {
		t.Fatalf("count %d", n)
	}
	path, n, _, err := st.ExportFeedback(dir, time.Now())
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

// Owner, 2026-11-16: an export takes only the remarks no earlier export had,
// and the exported ones can be cleared once passed on; a remark written
// after the export stays.
func TestFeedbackExportsNewAndClearsExported(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	dir := t.TempDir()
	for _, text := range []string{"old one", "old two"} {
		if _, err := st.AddFeedback(AudienceOwner, "", text, "", "", nil); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string {
		z, err := zip.OpenReader(path)
		if err != nil {
			t.Fatal(err)
		}
		defer z.Close()
		r, _ := z.File[len(z.File)-1].Open()
		b, _ := io.ReadAll(r)
		r.Close()
		return string(b)
	}
	if _, n, fresh, err := st.ExportFeedback(dir, time.Now()); err != nil || n != 2 || !fresh {
		t.Fatalf("first export %d %v %v", n, fresh, err)
	}
	if _, err := st.AddFeedback(AudienceCoordinator, "", "new one", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if all, fresh, _ := st.FeedbackCount(); all != 3 || fresh != 1 {
		t.Fatalf("count %d new %d", all, fresh)
	}
	path, n, fresh, err := st.ExportFeedback(dir, time.Now().Add(time.Minute))
	if md := read(path); err != nil || n != 1 || !fresh || !strings.Contains(md, "new one") || strings.Contains(md, "old one") {
		t.Fatalf("second export %d %v %v %q", n, fresh, err, md)
	}
	if _, err := st.AddFeedback(AudienceOwner, "", "after the export", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if deleted, left, err := st.ClearExportedFeedback(); err != nil || deleted != 3 || left != 1 {
		t.Fatalf("clear %d %d %v", deleted, left, err)
	}
	list, _ := st.FeedbackList()
	if len(list) != 1 || list[0].Text != "after the export" {
		t.Fatalf("left %+v", list)
	}
	// Nothing new: the export writes every remark again.
	if _, _, _, err := st.ExportFeedback(dir, time.Now().Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, n, fresh, err := st.ExportFeedback(dir, time.Now().Add(3*time.Minute)); err != nil || n != 1 || fresh {
		t.Fatalf("repeat export %d %v %v", n, fresh, err)
	}
}
