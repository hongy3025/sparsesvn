package plan

import (
	"strings"

	"github.com/hongy3025/sparsesvn/internal/config"
)

func Expand(cfg *config.Config) map[string]config.Depth {
	out := make(map[string]config.Depth, len(cfg.Paths)*2)
	for _, p := range cfg.Paths {
		out[p.Path] = p.Depth
	}
	for _, p := range cfg.Paths {
		parts := strings.Split(p.Path, "/")
		for i := 1; i < len(parts); i++ {
			parent := strings.Join(parts[:i], "/")
			if _, ok := out[parent]; !ok {
				out[parent] = config.DepthEmpty
			}
		}
	}
	return out
}
