package plan

import (
	"github.com/hongy3025/sparsesvn/internal/config"
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

// DiffWithExternals computes path-level and external-level diff.
// desiredExt and currentExt map parent path -> external specs.
func DiffWithExternals(
	desired, current map[string]config.Depth,
	desiredExt, currentExt map[string][]ExternalSpec,
) []Action {
	actions := Diff(desired, current)

	// For each path present in both desired and current, diff externals
	for path, desiredExts := range desiredExt {
		if _, inDesired := desired[path]; !inDesired {
			continue // parent excluded, handled below
		}
		currentExts := currentExt[path]
		actions = append(actions, diffExternals(path, desiredExts, currentExts)...)
	}

	// Parent path excluded -> auto-exclude all its externals
	for path, currentExts := range currentExt {
		if _, inDesired := desired[path]; !inDesired {
			for _, ext := range currentExts {
				actions = append(actions, Action{
					Kind:      ActionExclude,
					Path:      path,
					FromDepth: ext.Depth,
					External:  &ExternalAction{Target: ext.Target, ParentPath: path},
				})
			}
		}
	}

	return actions
}

// diffExternals computes external-level diff for a single parent path.
func diffExternals(parentPath string, desired, current []ExternalSpec) []Action {
	dMap := make(map[string]config.Depth, len(desired))
	for _, e := range desired {
		dMap[e.Target] = e.Depth
	}
	cMap := make(map[string]config.Depth, len(current))
	for _, e := range current {
		cMap[e.Target] = e.Depth
	}

	var actions []Action
	for target, dDepth := range dMap {
		cDepth, ok := cMap[target]
		if !ok {
			actions = append(actions, Action{
				Kind:     ActionAdd,
				Path:     parentPath,
				ToDepth:  dDepth,
				External: &ExternalAction{Target: target, ParentPath: parentPath},
			})
			continue
		}
		if dDepth == cDepth {
			continue
		}
		if dDepth > cDepth {
			actions = append(actions, Action{
				Kind:      ActionUpgrade,
				Path:      parentPath,
				FromDepth: cDepth,
				ToDepth:   dDepth,
				External:  &ExternalAction{Target: target, ParentPath: parentPath},
			})
		} else {
			actions = append(actions, Action{
				Kind:      ActionDowngrade,
				Path:      parentPath,
				FromDepth: cDepth,
				ToDepth:   dDepth,
				External:  &ExternalAction{Target: target, ParentPath: parentPath},
			})
		}
	}
	for target, cDepth := range cMap {
		if _, ok := dMap[target]; !ok {
			actions = append(actions, Action{
				Kind:      ActionExclude,
				Path:      parentPath,
				FromDepth: cDepth,
				External:  &ExternalAction{Target: target, ParentPath: parentPath},
			})
		}
	}
	return actions
}
