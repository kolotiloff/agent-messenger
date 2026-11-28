package platform

import (
	"errors"
	"os"
	"path/filepath"
)

const windowSetupFile = "window-setup"
const windowDefaultModelFile = "window-default-model"

// MarkFreshWindowSetup records that this data root did not have an installed
// queue before the first client run. Existing and migrated roots keep their UI.
func MarkFreshWindowSetup(root, queue string) error {
	if _, err := os.Stat(queue); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	path := filepath.Join(root, windowSetupFile)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = f.WriteString("pending")
	return errors.Join(err, f.Close())
}

func WindowSetupPending(root string) bool {
	raw, err := ReadFile(filepath.Join(root, windowSetupFile))
	return err == nil && string(raw) == "pending"
}

// DismissWindowSetup records a lasting opt-out without altering project data.
func DismissWindowSetup(root string) error {
	return writeWindowSetupPreference(root, windowSetupFile, "done")
}

func WindowDefaultModel(root string) string {
	raw, _ := ReadFile(filepath.Join(root, windowDefaultModelFile))
	return string(raw)
}

func SetWindowDefaultModel(root, model string) error {
	return writeWindowSetupPreference(root, windowDefaultModelFile, model)
}

func writeWindowSetupPreference(root, name, value string) error {
	f, err := os.CreateTemp(root, "."+name+"-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString(value)
	err = errors.Join(err, f.Sync(), f.Close())
	if err != nil {
		return err
	}
	return ReplaceFile(f.Name(), filepath.Join(root, name))
}
