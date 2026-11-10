package supervisor

import (
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// PinnedRuntimeVariable names the pinned copy of this program (one folder
// per version) when the client was started from the stable runtime file
// (D-118). Workers and their MCP processes start the pinned copy, so a later
// update that replaces the stable file does not change their program.
const PinnedRuntimeVariable = "AGENT_MESSENGER_PINNED_RUNTIME"

var workerProgramOnce struct {
	sync.Once
	path string
	err  error
}

// workerProgram is the program file workers start: the pinned copy named by
// PinnedRuntimeVariable when it holds exactly this program's bytes,
// otherwise this program itself.
func workerProgram() (string, error) {
	workerProgramOnce.Do(func() {
		workerProgramOnce.path, workerProgramOnce.err = resolveWorkerProgram(os.Getenv(PinnedRuntimeVariable))
	})
	return workerProgramOnce.path, workerProgramOnce.err
}

func resolveWorkerProgram(pinned string) (string, error) {
	self, err := os.Executable()
	if err != nil || pinned == "" || !filepath.IsAbs(pinned) {
		return self, err
	}
	if st, err := os.Stat(pinned); err != nil || !st.Mode().IsRegular() {
		return self, nil
	}
	a, errA := digestOf(self)
	b, errB := digestOf(pinned)
	if errA != nil || errB != nil || a != b {
		return self, nil
	}
	return pinned, nil
}

func digestOf(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return string(h.Sum(nil)), nil
}
