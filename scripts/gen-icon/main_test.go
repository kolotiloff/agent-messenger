package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// The committed resource files are what this program writes now.
func TestSysoUpToDate(t *testing.T) {
	for name, data := range map[string][]byte{
		"rsrc_windows_amd64.syso": syso(0x8664, 0x0003),
		"rsrc_windows_arm64.syso": syso(0xAA64, 0x0002),
	} {
		committed, err := os.ReadFile(filepath.Join("..", "..", "cmd", "messenger", name))
		if err != nil || !bytes.Equal(committed, data) {
			t.Fatalf("%s is stale; run go run ./scripts/gen-icon cmd/messenger (%v)", name, err)
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
