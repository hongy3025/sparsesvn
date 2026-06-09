package plan

import (
	"github.com/sparsesvn/sparsesvn/internal/config"
)

func Diff(desired, current map[string]config.Depth) []Action {
	actions := make([]Action, 0)

	for p, dDepth := range desired {
		cDepth, ok := current[p]
		if !ok {
			actions = append(actions, Action{Kind: ActionAdd, Path: p, ToDepth: dDepth})
			continue
		}
		if dDepth == cDepth {
			continue
		}
		if dDepth > cDepth {
			actions = append(actions, Action{Kind: ActionUpgrade, Path: p, FromDepth: cDepth, ToDepth: dDepth})
		} else {
			actions = append(actions, Action{Kind: ActionDowngrade, Path: p, FromDepth: cDepth, ToDepth: dDepth})
		}
	}

	for p, cDepth := range current {
		if _, ok := desired[p]; !ok {
			actions = append(actions, Action{Kind: ActionExclude, Path: p, FromDepth: cDepth})
		}
	}

	return actions
}
