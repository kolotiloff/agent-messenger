package main

import (
	"bytes"
	"encoding/binary"
	"image"
	stddraw "image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The committed resource files are what this program writes now. The PNG
// encoder's bytes change between Go versions, so the embedded PNG is checked
// by its pixels and the rest of the file byte for byte.
func TestSysoUpToDate(t *testing.T) {
	defer func() { encodedPNG = nil }()
	for _, f := range []struct {
		name               string
		machine, relocType uint16
	}{{"rsrc_windows_amd64.syso", 0x8664, 0x0003}, {"rsrc_windows_arm64.syso", 0xAA64, 0x0002}} {
		committed, err := os.ReadFile(filepath.Join("..", "..", "cmd", "messenger", f.name))
		if err != nil {
			t.Fatal(err)
		}
		start := bytes.Index(committed, []byte("\x89PNG\r\n\x1a\n"))
		end := bytes.Index(committed, []byte("IEND"))
		if start < 0 || end < start {
			t.Fatalf("%s has no PNG", f.name)
		}
		embedded := committed[start : end+8]
		decoded, err := png.Decode(bytes.NewReader(embedded))
		if err != nil {
			t.Fatalf("%s: %v", f.name, err)
		}
		want := draw(256)
		got := image.NewNRGBA(want.Bounds())
		stddraw.Draw(got, got.Bounds(), decoded, image.Point{}, stddraw.Src)
		if !bytes.Equal(got.Pix, want.Pix) {
			t.Fatalf("%s: the PNG is stale; run go run ./scripts/gen-icon cmd/messenger", f.name)
		}
		encodedPNG = embedded
		if !bytes.Equal(committed, syso(f.machine, f.relocType)) {
			t.Fatalf("%s is stale; run go run ./scripts/gen-icon cmd/messenger", f.name)
		}
	}
}

// Every data entry points at its image: the first icon is the 256 px PNG,
// the group lists all sizes.
func TestResourceDirectory(t *testing.T) {
	section, relocs := resources()
	if len(relocs) != len(iconSizes)+1 {
		t.Fatalf("%d relocations", len(relocs))
	}
	first := binary.LittleEndian.Uint32(section[relocs[0]:])
	if !bytes.HasPrefix(section[first:], []byte("\x89PNG")) {
		t.Fatal("first icon is not the PNG")
	}
	group := binary.LittleEndian.Uint32(section[relocs[len(relocs)-1]:])
	if n := binary.LittleEndian.Uint16(section[group+4:]); int(n) != len(iconSizes) {
		t.Fatalf("group lists %d images", n)
	}
	small := binary.LittleEndian.Uint32(section[relocs[len(iconSizes)-1]:])
	if h := binary.LittleEndian.Uint32(section[small+8:]); h != 32 {
		t.Fatalf("16 px bitmap height %d", h)
	}
}
