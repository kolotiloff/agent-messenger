package core

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8((x*7919 + y*104729) ^ (x * y)), 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestWorkerPicturesArePublishedWithTheReport(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op-1", "claude", "draw the level")
	r := f.launch(t, id, f.sessionOf(t, id))
	if _, err := f.st.ReadTask(r.ID, r.Receipt, false); err != nil {
		t.Fatal(err)
	}
	pic := testPNG(t, 8, 8)
	for _, bad := range [][]TaskImageFile{
		{{Name: "notes.txt", Data: []byte("plain text, not a picture")}},
		{{Name: "a.png", Data: pic}, {Name: "b.png", Data: pic}, {Name: "c.png", Data: pic}, {Name: "d.png", Data: pic}, {Name: "e.png", Data: pic}},
		{{Name: "empty.png"}},
	} {
		if err := f.st.RecordDispositionImages(r.ID, r.Receipt, DispositionReport, "done", bad); !errors.Is(err, ErrRejected) {
			t.Fatalf("accepted %d bad pictures: %v", len(bad), err)
		}
	}
	if err := f.st.RecordDispositionImages(r.ID, r.Receipt, DispositionReport, "Level drawn", []TaskImageFile{{Name: "/x/shots/level.png", Data: pic}}); err != nil {
		t.Fatal(err)
	}
	// Not published before the run is verified.
	if list, err := f.st.TaskImages(id, 4); err != nil || len(list) != 0 {
		t.Fatalf("pictures before verification: %+v %v", list, err)
	}
	if state, err := f.st.FinishTaskRun(r.ID, cleanExit); err != nil || state != "finished" {
		t.Fatalf("finish %s %v", state, err)
	}
	list, err := f.st.TaskImages(id, 4)
	if err != nil || len(list) != 1 || list[0].Name != "level.png" || list[0].Type != "image/png" || list[0].Size != len(pic) {
		t.Fatalf("pictures %+v %v", list, err)
	}
	kind, data, err := f.st.TaskImageData(list[0].ID)
	if err != nil || kind != "image/png" || !bytes.Equal(data, pic) {
		t.Fatalf("data %s %d %v", kind, len(data), err)
	}
	var report TaskEvent
	for _, e := range f.events(t, AudienceCoordinator, 0) {
		if e.Kind == "report" {
			report = e
		}
	}
	if !strings.Contains(report.Text, "1 picture(s) attached") {
		t.Fatalf("report does not mention the picture: %q", report.Text)
	}
	feed, err := f.st.TaskFeed(id)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range feed.Items {
		if it.Kind == "report" && len(it.Images) == 1 && it.Images[0].ID == list[0].ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("feed lacks the picture: %+v", feed.Items)
	}
}

func TestWorkerPicturesReachTheComputerThatGaveTheTask(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op1", "codex", "Draw level 1")
	p.carry(t, true)
	m := p.mirror(t, id)
	p.carry(t, false)
	r := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000c1")
	if _, err := p.b.st.ReadTask(r.ID, r.Receipt, false); err != nil {
		t.Fatal(err)
	}
	small := testPNG(t, 8, 8)
	// A noisy picture far over one message: it goes as a smaller JPEG.
	big := testPNG(t, 1400, 1000)
	if len(big) <= maxNetImage {
		t.Fatalf("test picture too small: %d", len(big))
	}
	if err := p.b.st.RecordDispositionImages(r.ID, r.Receipt, DispositionReport, "Level 1 drawn", []TaskImageFile{{Name: "small.png", Data: small}, {Name: "big.png", Data: big}}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.b.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	list, err := p.a.st.TaskImages(id, 4)
	if err != nil || len(list) != 2 {
		t.Fatalf("pictures on a %+v %v", list, err)
	}
	if list[0].Name != "small.png" || list[0].Type != "image/png" || list[0].Size != len(small) {
		t.Fatalf("small %+v", list[0])
	}
	if list[1].Name != "big.png" || list[1].Type != "image/jpeg" || list[1].Size > maxNetImage {
		t.Fatalf("big %+v", list[1])
	}
	feed, err := p.a.st.TaskFeed(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range feed.Items {
		if it.Kind == "report" {
			if len(it.Images) != 2 {
				t.Fatalf("report pictures %+v", it.Images)
			}
			return
		}
	}
	t.Fatalf("no report in %+v", feed.Items)
}
