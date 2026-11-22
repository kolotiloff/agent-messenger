package localui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

const machineOrderRoute = "/api/machine-order"

var machineOrderKey = regexp.MustCompile(`^[a-zA-Z0-9._:-]{1,80}$`)

type machineOrder struct {
	Order []string `json:"order"`
}

func (o machineOrder) valid() bool {
	if o.Order == nil || len(o.Order) > 512 {
		return false
	}
	seen := make(map[string]bool, len(o.Order))
	for _, key := range o.Order {
		if !machineOrderKey.MatchString(key) || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

func serveMachineOrder(w http.ResponseWriter, r *http.Request, root string, decoder *json.Decoder) {
	if root == "" {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(root, "computer-order.json")
	var order machineOrder
	if decoder == nil {
		raw, err := platform.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, order)
			return
		}
		if err != nil || json.Unmarshal(raw, &order) != nil || !order.valid() {
			http.Error(w, "computer order unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, order)
		return
	}
	if err := decoder.Decode(&order); err != nil || decoder.Decode(new(any)) != io.EOF || !order.valid() {
		http.Error(w, "invalid computer order", http.StatusBadRequest)
		return
	}
	raw, err := json.Marshal(order)
	if err != nil || writeMachineOrder(path, raw) != nil {
		http.Error(w, "could not save computer order", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func writeMachineOrder(path string, raw []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".computer-order-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(raw)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := platform.ReplaceFile(tmp.Name(), path); err != nil {
		return err
	}
	return platform.SyncDir(dir)
}
