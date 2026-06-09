package state

import (
	"path/filepath"
	"time"

	"github.com/sparsesvn/sparsesvn/internal/config"
)

const StateVersion = 1

const StateFileRelPath = ".svn/sparsesvn.state.yaml"

type PathEntry struct {
	Path  string
	Depth config.Depth
}

type State struct {
	Version    int
	ConfigHash string
	URL        string
	AppliedAt  time.Time
	Paths      []PathEntry
}

func Path(workdir string) string {
	return filepath.Join(workdir, StateFileRelPath)
}
