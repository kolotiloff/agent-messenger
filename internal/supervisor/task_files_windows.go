package supervisor

import (
	"os"

	"github.com/google/safeopen"
)

func openTaskFile(root, path string) (*os.File, error) {
	return safeopen.OpenBeneath(root, path)
}
