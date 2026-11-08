// Command gen-icon draws the program's icon (the same shapes as
// desktop/macos/icon.swift and internal/localui/assets/icon.svg) and writes
// it as Windows program resources: cmd/messenger/rsrc_windows_<arch>.syso,
// which the Go linker puts into messenger.exe. Explorer, the taskbar and the
// window then show it. Only the standard library is used; edges are
// smoothed by sampling each pixel 4×4 times.
//
//	go run ./scripts/gen-icon cmd/messenger
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

type shape struct {
	inside func(x, y float64) bool
	c      color.NRGBA
}

func roundRect(x, y, w, h, r float64) func(float64, float64) bool {
	return func(px, py float64) bool {
		if px < x || px > x+w || py < y || py > y+h {
			return false
		}
		cx := min(max(px, x+r), x+w-r)
		cy := min(max(py, y+r), y+h-r)
		dx, dy := px-cx, py-cy
		return dx*dx+dy*dy <= r*r
	}
}

// triangle is the speech bubble's tail: (132,322), (132,387), (208,322).
func triangle(px, py float64) bool {
	if px < 132 || py < 322 {
		return false
	}
	// Below the edge from (132,387) to (208,322).
	return (py-387)*(208-132)-(px-132)*(322-387) <= 0
}

var shapes = []shape{
	{roundRect(24, 24, 464, 464, 104), color.NRGBA{0x1F, 0x24, 0x30, 0xFF}},
	{roundRect(94, 128, 260, 207, 44), color.NRGBA{0x85, 0xA8, 0xF0, 0xFF}},
	{triangle, color.NRGBA{0x85, 0xA8, 0xF0, 0xFF}},
	{roundRect(235, 269, 182, 143, 32), color.NRGBA{0xF2, 0xF2, 0xF2, 0xFF}},
}

func draw(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const n = 4
	scale := 512 / float64(size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a float64
			for sy := 0; sy < n; sy++ {
				for sx := 0; sx < n; sx++ {
					px := (float64(x) + (float64(sx)+0.5)/n) * scale
					py := (float64(y) + (float64(sy)+0.5)/n) * scale
					for i := len(shapes) - 1; i >= 0; i-- {
						if shapes[i].inside(px, py) {
							c := shapes[i].c
							r, g, b, a = r+float64(c.R), g+float64(c.G), b+float64(c.B), a+255
							break
						}
					}
				}
			}
			if a == 0 {
				continue
			}
			k := a / 255
			img.SetNRGBA(x, y, color.NRGBA{uint8(r / k), uint8(g / k), uint8(b / k), uint8(a / (n * n))})
		}
	}
	return img
}

// iconSizes are the images of the icon; 256 is stored as PNG, the others as
// 32-bit bitmaps, as Windows expects.
var iconSizes = []int{256, 128, 64, 48, 32, 24, 16}

// IconID is the resource ID of the icon group; the window loads it by this
// number (window_windows.go).
const IconID = 1

func iconImage(size int) []byte {
	img := draw(size)
	if size == 256 {
		var buf bytes.Buffer
		_ = png.Encode(&buf, img)
		return buf.Bytes()
	}
	var buf bytes.Buffer
	le := func(v any) { _ = binary.Write(&buf, binary.LittleEndian, v) }
	// BITMAPINFOHEADER; the height counts the colour and the mask rows.
	le(uint32(40))
	le(int32(size))
	le(int32(size * 2))
	le(uint16(1))
	le(uint16(32))
	le(uint32(0))
	le(uint32(size * size * 4))
	le(int32(0))
	le(int32(0))
	le(uint32(0))
	le(uint32(0))
	for y := size - 1; y >= 0; y-- {
		for x := 0; x < size; x++ {
			c := img.NRGBAAt(x, y)
			buf.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	// The AND mask: all zero (the alpha channel decides), rows padded to 4 bytes.
	buf.Write(make([]byte, ((size+31)/32)*4*size))
	return buf.Bytes()
}

// resources builds the .rsrc section: RT_ICON (3) entries 1..n and one
// RT_GROUP_ICON (14) entry. It returns the section and the offsets of the
// data entries' address fields, which need ADDR32NB relocations.
func resources() ([]byte, []uint32) {
	images := make([][]byte, len(iconSizes))
	for i, size := range iconSizes {
		images[i] = iconImage(size)
	}
	var group bytes.Buffer
	_ = binary.Write(&group, binary.LittleEndian, []uint16{0, 1, uint16(len(images))})
	for i, size := range iconSizes {
		w := byte(size)
		if size == 256 {
			w = 0
		}
		group.Write([]byte{w, w, 0, 0})
		_ = binary.Write(&group, binary.LittleEndian, uint16(1))
		_ = binary.Write(&group, binary.LittleEndian, uint16(32))
		_ = binary.Write(&group, binary.LittleEndian, uint32(len(images[i])))
		_ = binary.Write(&group, binary.LittleEndian, uint16(i+1))
	}
	type leaf struct {
		typ, id uint16
		data    []byte
	}
	var leaves []leaf
	for i, data := range images {
		leaves = append(leaves, leaf{3, uint16(i + 1), data})
	}
	leaves = append(leaves, leaf{14, IconID, group.Bytes()})

	const lang = 0x0409
	dirSize := func(entries int) int { return 16 + 8*entries }
	// Layout: root, type directories, name directories (one per leaf),
	// data entries, then the data.
	root := 0
	typeDirs := map[uint16]int{}
	off := dirSize(2)
	typeDirs[3] = off
	off += dirSize(len(images))
	typeDirs[14] = off
	off += dirSize(1)
	nameDirs := make([]int, len(leaves))
	for i := range leaves {
		nameDirs[i] = off
		off += dirSize(1)
	}
	dataEntries := make([]int, len(leaves))
	for i := range leaves {
		dataEntries[i] = off
		off += 16
	}
	dataAt := make([]int, len(leaves))
	for i, l := range leaves {
		off = (off + 7) &^ 7
		dataAt[i] = off
		off += len(l.data)
	}
	out := make([]byte, (off+7)&^7)
	put16 := func(at int, v uint16) { binary.LittleEndian.PutUint16(out[at:], v) }
	put32 := func(at int, v uint32) { binary.LittleEndian.PutUint32(out[at:], v) }
	dir := func(at, ids int) { put16(at+14, uint16(ids)) }
	entry := func(at, i int, id uint32, target int, sub bool) {
		e := at + 16 + 8*i
		put32(e, id)
		v := uint32(target)
		if sub {
			v |= 0x80000000
		}
		put32(e+4, v)
	}
	dir(root, 2)
	entry(root, 0, 3, typeDirs[3], true)
	entry(root, 1, 14, typeDirs[14], true)
	dir(typeDirs[3], len(images))
	dir(typeDirs[14], 1)
	var relocs []uint32
	n3 := 0
	for i, l := range leaves {
		if l.typ == 3 {
			entry(typeDirs[3], n3, uint32(l.id), nameDirs[i], true)
			n3++
		} else {
			entry(typeDirs[14], 0, uint32(l.id), nameDirs[i], true)
		}
		dir(nameDirs[i], 1)
		entry(nameDirs[i], 0, lang, dataEntries[i], false)
		put32(dataEntries[i], uint32(dataAt[i])) // RVA once relocated
		put32(dataEntries[i]+4, uint32(len(l.data)))
		relocs = append(relocs, uint32(dataEntries[i]))
		copy(out[dataAt[i]:], l.data)
	}
	return out, relocs
}

// syso wraps the section in a COFF object with one section symbol.
func syso(machine uint16, relocType uint16) []byte {
	section, relocs := resources()
	var b bytes.Buffer
	le := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	const headers = 20 + 40
	relocAt := headers + len(section)
	symAt := relocAt + 10*len(relocs)
	le(machine)
	le(uint16(1))
	le(uint32(0))
	le(uint32(symAt))
	le(uint32(1))
	le(uint16(0))
	le(uint16(0))
	b.WriteString(".rsrc\x00\x00\x00")
	le(uint32(0))
	le(uint32(0))
	le(uint32(len(section)))
	le(uint32(headers))
	le(uint32(relocAt))
	le(uint32(0))
	le(uint16(len(relocs)))
	le(uint16(0))
	le(uint32(0x40000040)) // initialized data, readable
	b.Write(section)
	for _, r := range relocs {
		le(r)
		le(uint32(0))
		le(relocType)
	}
	b.WriteString(".rsrc\x00\x00\x00")
	le(uint32(0))
	le(int16(1))
	le(uint16(0))
	b.WriteByte(3) // static
	b.WriteByte(0)
	le(uint32(4)) // empty string table
	return b.Bytes()
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gen-icon <dir>")
		os.Exit(2)
	}
	for name, data := range map[string][]byte{
		"rsrc_windows_amd64.syso": syso(0x8664, 0x0003), // IMAGE_REL_AMD64_ADDR32NB
		"rsrc_windows_arm64.syso": syso(0xAA64, 0x0002), // IMAGE_REL_ARM64_ADDR32NB
	} {
		if err := os.WriteFile(filepath.Join(os.Args[1], name), data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
