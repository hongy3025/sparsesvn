package plan

import (
	"sort"
	"strings"
)

func pathDepth(p string) int {
	return strings.Count(p, "/")
}

// sortPath returns the effective path for sorting purposes.
// For external actions, this is parentPath/target (one level deeper).
func sortPath(a Action) string {
	if a.External != nil {
		return a.External.ParentPath + "/" + a.External.Target
	}
	return a.Path
}

func Sort(actions []Action) {
	sort.SliceStable(actions, func(i, j int) bool {
		a, b := actions[i], actions[j]
		aGroup := groupOf(a.Kind)
		bGroup := groupOf(b.Kind)
		if aGroup != bGroup {
			return aGroup < bGroup
		}
		aSortPath := sortPath(a)
		bSortPath := sortPath(b)
		aDepth := pathDepth(aSortPath)
		bDepth := pathDepth(bSortPath)
		if aGroup == 0 {
			if aDepth != bDepth {
				return aDepth < bDepth
			}
		} else {
			if aDepth != bDepth {
				return aDepth > bDepth
			}
		}
		return aSortPath < bSortPath
	})
}

func groupOf(k ActionKind) int {
	switch k {
	case ActionAdd, ActionUpgrade:
		return 0
	default:
		return 1
	}
}
