package core

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"errors"
	"image"
	"image/draw"
	_ "image/gif" // decodes GIF pictures for sending
	"image/jpeg"
	_ "image/png" // decodes PNG pictures for sending
	"net/http"
)

// T-175 between computers: a picture goes to the computer that gave the
// task in one network message, before the report or question it belongs
// to. A message holds at most maxNetImage bytes, so a larger picture is sent
// as a smaller JPEG. A picture that cannot be made small enough (a large
// WebP, which the standard library cannot read) is not sent; the report
// still arrives. A computer before this version ignores the message.

const (
	maxNetImage      = 180 << 10
	maxImagesPerTask = 40
)

// forwardImages sends a run's pictures to the computer that gave its task.
func forwardImages(tx *sql.Tx, task, run string) error {
	link, err := taskLink(tx, task)
	if err != nil || link.origin == nil {
		return err
	}
	rows, err := tx.Query(`SELECT id,name,type,data FROM task_images WHERE run_id=? ORDER BY seq`, run)
	if err != nil {
		return err
	}
	type pic struct {
		id, name, kind string
		data           []byte
	}
	var list []pic
	for rows.Next() {
		var p pic
		if err := rows.Scan(&p.id, &p.name, &p.kind, &p.data); err != nil {
			rows.Close()
			return err
		}
		list = append(list, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range list {
		kind, data, ok := netImage(p.kind, p.data)
		if !ok {
			continue
		}
		body := netBody{Kind: "image", Task: link.origin.task, Title: p.name, Detail: kind, Text: base64.StdEncoding.EncodeToString(data)}
		if err := enqueueNet(tx, link.origin.broker, link.origin.peer, "image."+p.id, task, body); err != nil {
			return err
		}
	}
	return nil
}

// netImage fits a picture into one message.
func netImage(kind string, data []byte) (string, []byte, bool) {
	if len(data) <= maxNetImage {
		return kind, data, true
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", nil, false
	}
	for _, side := range []int{1600, 1200, 900, 640} {
		for _, quality := range []int{80, 65, 50} {
			var buf bytes.Buffer
			if jpeg.Encode(&buf, scaleTo(src, side), &jpeg.Options{Quality: quality}) != nil {
				return "", nil, false
			}
			if buf.Len() <= maxNetImage {
				return "image/jpeg", buf.Bytes(), true
			}
		}
	}
	return "", nil, false
}

// scaleTo shrinks a picture so its longer side is at most side, by
// averaging the source pixels under each new pixel. It never enlarges.
func scaleTo(src image.Image, side int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= side && h <= side {
		rgba := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)
		return rgba
	}
	nw, nh := side, h*side/w
	if h > w {
		nw, nh = w*side/h, side
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		y0, y1 := b.Min.Y+y*h/nh, b.Min.Y+(y+1)*h/nh
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < nw; x++ {
			x0, x1 := b.Min.X+x*w/nw, b.Min.X+(x+1)*w/nw
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, bl, a, n uint64
			// Sample at most a 4x4 grid of the source block: enough for a
			// preview and fast on large screenshots.
			sy, sx := max(1, (y1-y0)/4), max(1, (x1-x0)/4)
			for yy := y0; yy < y1; yy += sy {
				for xx := x0; xx < x1; xx += sx {
					cr, cg, cb, ca := src.At(xx, yy).RGBA()
					r, g, bl, a, n = r+uint64(cr), g+uint64(cg), bl+uint64(cb), a+uint64(ca), n+1
				}
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = uint8(r/n>>8), uint8(g/n>>8), uint8(bl/n>>8), uint8(a/n>>8)
		}
	}
	return dst
}

// applyImage keeps a picture the computer that runs one of this computer's
// tasks sent with its next report or question.
func applyImage(tx *sql.Tx, from netLink, b netBody) error {
	var task string
	err := tx.QueryRow(`SELECT id FROM tasks WHERE id=? AND remote_broker=? AND remote_peer=?`, from.task, from.broker, from.peer).Scan(&task)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(b.Text)
	if err != nil || len(data) == 0 || len(data) > maxNetImage {
		return nil
	}
	kind := http.DetectContentType(data)
	if !TaskImageTypes[kind] {
		return nil
	}
	var kept, events int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM task_images WHERE task_id=?`, task).Scan(&kept); err != nil {
		return err
	}
	if kept >= maxImagesPerTask {
		return nil
	}
	// The picture belongs to the next report or question of the task.
	if err := tx.QueryRow(`SELECT COUNT(*) FROM task_events WHERE task_id=? AND kind IN ('report','late_report','question')`, task).Scan(&events); err != nil {
		return err
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO task_images(id,task_id,run_id,event_seq,seq,name,type,data,created_at) VALUES (?,?,'',?,?,?,?,?,?)`,
		"img-"+id[:20], task, events+1, kept+1, clip(b.Title, 120), kind, data, now())
	return err
}
