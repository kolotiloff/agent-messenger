package supervisor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
			if _, statErr := os.Lstat(run.Root); errors.Is(statErr, fs.ErrNotExist) {
				return "", fmt.Errorf("каталог данных из защитного реестра больше не существует (%s); после перезагрузки снимите резерв командой messenger recover-missing-root --confirm preserve-uncertain", run.Root)
			}
			return "", err
		}
		if chosen != "" && chosen != root {
			return "", errors.New("несколько каталогов в защитном реестре; выберите существующий каталог данных явно")
		}
		if isTemporaryRoot(root) {
			return "", fmt.Errorf("защитный реестр указывает на временный каталог (%s); macOS удаляет его при перезагрузке, поэтому окно не использует его для данных. Выберите каталог данных явно или дождитесь завершения тестового запуска", root)
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

// isTemporaryRoot reports whether a canonical directory lies in a system
// temporary folder, which the OS may empty on reboot. Tests override it.
var isTemporaryRoot = func(root string) bool {
	candidates := []string{os.TempDir(), "/tmp", "/private/tmp", "/var/folders", "/private/var/folders", "/var/tmp", "/private/var/tmp"}
	for _, dir := range candidates {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			candidates = append(candidates, resolved)
		}
	}
	for _, dir := range candidates {
		dir = filepath.Clean(dir)
		if dir == "" || dir == "/" {
			continue
		}
		if root == dir || strings.HasPrefix(root, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
