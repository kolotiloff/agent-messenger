package supervisor

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// DesktopRoot selects the existing reservation's database, never a replacement
// database to evade it. Reading this hint grants no ownership or recovery rights.
func DesktopRoot() (string, error) {
	runs, err := loadRuns()
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return platform.DefaultRoot()
	}
	chosen := ""
	for _, run := range runs {
		root, err := core.CanonicalEnvironment(run.Root)
		if err != nil {
			return "", err
		}
		if chosen != "" && chosen != root {
			return "", errors.New("несколько каталогов в защитном реестре; выберите существующий каталог данных явно")
		}
		info, err := os.Stat(filepath.Join(root, dbName))
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", errors.New("сохранённая база недоступна")
		}
		chosen = root
	}
	return chosen, nil
}
