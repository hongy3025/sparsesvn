package plan

import (
	"sort"
	"strings"
)

func pathDepth(p string) int {
	return strings.Count(p, "/")
}

func Sort(actions []Action) {
	sort.SliceStable(actions, func(i, j int) bool {
		a, b := actions[i], actions[j]
		aGroup := groupOf(a.Kind)
		bGroup := groupOf(b.Kind)
		if aGroup != bGroup {
			return aGroup < bGroup
		}
		aDepth := pathDepth(a.Path)
		bDepth := pathDepth(b.Path)
		if aGroup == 0 {
			if aDepth != bDepth {
				return aDepth < bDepth
			}
		} else {
			if aDepth != bDepth {
				return aDepth > bDepth
			}
		}
		return a.Path < b.Path
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
