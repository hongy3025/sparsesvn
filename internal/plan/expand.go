package plan

import (
	"strings"

	"github.com/hongy3025/sparsesvn/internal/config"
)

type ExpandResult struct {
	Paths     map[string]config.Depth
	Externals map[string][]ExternalSpec
}

type ExternalSpec struct {
	Target string
	Depth  config.Depth
}

func Expand(cfg *config.Config) *ExpandResult {
	out := make(map[string]config.Depth, len(cfg.Paths)*2)
	extMap := make(map[string][]ExternalSpec, len(cfg.Paths))

	for _, p := range cfg.Paths {
		out[p.Path] = p.Depth
		var exts []ExternalSpec
		for _, e := range p.Externals {
			exts = append(exts, ExternalSpec{Target: e.Target, Depth: e.Depth})
		}
		if exts == nil {
			exts = []ExternalSpec{}
		}
		extMap[p.Path] = exts
	}
	for _, p := range cfg.Paths {
		parts := strings.Split(p.Path, "/")
		for i := 1; i < len(parts); i++ {
			parent := strings.Join(parts[:i], "/")
			if _, ok := out[parent]; !ok {
				out[parent] = config.DepthEmpty
			}
			if _, ok := extMap[parent]; !ok {
				extMap[parent] = []ExternalSpec{}
			}
		}
	}
	return &ExpandResult{Paths: out, Externals: extMap}
}