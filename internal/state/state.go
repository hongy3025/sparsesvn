package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/hongy3025/sparsesvn/internal/config"
)

const StateVersion = 2

const StateFileRelPath = ".svn/sparsesvn.state.yaml"

const stateFileHeader = "# sparsesvn state file - DO NOT EDIT MANUALLY\n"

type ExternalEntry struct {
	Target string
	Depth  config.Depth
}

type PathEntry struct {
	Path      string
	Depth     config.Depth
	Externals []ExternalEntry
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

type rawExternalEntry struct {
	Target string `yaml:"target"`
	Depth  string `yaml:"depth"`
}

type rawPathEntry struct {
	Path      string             `yaml:"path"`
	Depth     string             `yaml:"depth"`
	Externals []rawExternalEntry `yaml:"externals"`
}

type rawState struct {
	Version    int            `yaml:"version"`
	ConfigHash string         `yaml:"config_hash"`
	URL        string         `yaml:"url"`
	AppliedAt  time.Time      `yaml:"applied_at"`
	Paths      []rawPathEntry `yaml:"paths"`
}

func Save(workdir string, s *State) error {
	path := Path(workdir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	sorted := make([]PathEntry, len(s.Paths))
	copy(sorted, s.Paths)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	raw := rawState{
		Version:    s.Version,
		ConfigHash: s.ConfigHash,
		URL:        s.URL,
		AppliedAt:  s.AppliedAt.UTC(),
		Paths:      make([]rawPathEntry, len(sorted)),
	}
	for i, p := range sorted {
		rpe := rawPathEntry{Path: p.Path, Depth: p.Depth.String()}
		for _, e := range p.Externals {
			rpe.Externals = append(rpe.Externals, rawExternalEntry{Target: e.Target, Depth: e.Depth.String()})
		}
		raw.Paths[i] = rpe
	}

	body, err := yaml.Marshal(&raw)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	data := append([]byte(stateFileHeader), body...)

	if err := writeAtomic(path, data, 0644); err != nil {
		return fmt.Errorf("write state %s: %w", path, err)
	}
	return nil
}

func Load(workdir string) (*State, bool, error) {
	path := Path(workdir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read state %s: %w", path, err)
	}

	var raw rawState
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, false, fmt.Errorf("parse state %s: %w (consider deleting the state file to trigger full rebuild)", path, err)
	}

	if raw.Version > StateVersion {
		return nil, false, fmt.Errorf("state %s has version %d, max supported %d: please upgrade sparsesvn", path, raw.Version, StateVersion)
	}

	s := &State{
		Version:    raw.Version,
		ConfigHash: raw.ConfigHash,
		URL:        raw.URL,
		AppliedAt:  raw.AppliedAt,
		Paths:      make([]PathEntry, 0, len(raw.Paths)),
	}
	for i, rp := range raw.Paths {
		d, err := config.ParseDepth(rp.Depth)
		if err != nil {
			return nil, false, fmt.Errorf("parse state %s: paths[%d]: %w (consider deleting the state file to trigger full rebuild)", path, i, err)
		}
		pe := PathEntry{Path: rp.Path, Depth: d}
		for j, re := range rp.Externals {
			ed, err := config.ParseDepth(re.Depth)
			if err != nil {
				return nil, false, fmt.Errorf("parse state %s: paths[%d].externals[%d]: %w (consider deleting the state file to trigger full rebuild)", path, i, j, err)
			}
			pe.Externals = append(pe.Externals, ExternalEntry{Target: re.Target, Depth: ed})
		}
		s.Paths = append(s.Paths, pe)
	}
	return s, true, nil
}
