package executor

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hongy3025/sparsesvn/internal/config"
	"github.com/hongy3025/sparsesvn/internal/logx"
	"github.com/hongy3025/sparsesvn/internal/plan"
	"github.com/hongy3025/sparsesvn/internal/state"
	"github.com/hongy3025/sparsesvn/internal/svn"
)

type Options struct {
	ConfigPath  string
	Workdir     string
	URLOverride string
	Revision    string
	DryRun      bool
	Client      svn.Client
	Logger      *logx.Logger
}

type Result struct {
	Plan         []plan.Action
	ExecutedCount int
	FastPath     bool
	StateAfter   *state.State
	FailedAction *plan.Action
	Err          error
}

// Apply is the core entry point: load config -> load state -> validate url -> fast path -> expand -> diff -> sort
// -> top-level checkout (if needed) -> execute actions -> write state.
func Apply(ctx context.Context, opts Options) *Result {
	r := &Result{}

	// Step 1: load config
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		r.Err = fmt.Errorf("%w: %w", ErrConfigInvalid, err)
		return r
	}

	// Step 2: compute finalURL
	finalURL := cfg.URL
	if opts.URLOverride != "" {
		finalURL = opts.URLOverride
	}
	if finalURL == "" {
		r.Err = fmt.Errorf("%w", ErrURLRequired)
		return r
	}

	// Step 3: load state
	st, exists, err := state.Load(opts.Workdir)
	if err != nil {
		r.Err = fmt.Errorf("load state: %w", err)
		return r
	}

	// Step 4: url mismatch check
	if exists && st.URL != finalURL {
		r.Err = fmt.Errorf("%w: state has %q, config/override has %q", ErrURLMismatch, st.URL, finalURL)
		return r
	}

	// Step 5: compute configHash
	configHash, err := config.HashFile(opts.ConfigPath)
	if err != nil {
		r.Err = fmt.Errorf("hash config: %w", err)
		return r
	}

	// Step 6: fast path
	if exists && st.ConfigHash == configHash && opts.Revision == "" {
		r.FastPath = true
		r.ExecutedCount = 0
		r.StateAfter = st
		return r
	}

	// Step 7: expand + build current map
	expandResult := plan.Expand(cfg)
	desired := expandResult.Paths
	current := make(map[string]config.Depth)
	if exists {
		for _, p := range st.Paths {
			current[p.Path] = p.Depth
		}
	}

	// Step 8: diff + sort
	actions := plan.Diff(desired, current)
	plan.Sort(actions)
	r.Plan = actions

	// Step 9: actions empty + revision set -> UpdateRoot
	if len(actions) == 0 && opts.Revision != "" {
		if err := svn.UpdateRoot(ctx, opts.Client, opts.Workdir, opts.Revision); err != nil {
			r.Err = fmt.Errorf("update root: %w", err)
			return r
		}
		r.ExecutedCount = 1
		newState := buildState(configHash, finalURL, current)
		if saveErr := state.Save(opts.Workdir, newState); saveErr != nil {
			r.Err = fmt.Errorf("save state: %w", saveErr)
		}
		r.StateAfter = newState
		return r
	}

	// Step 10: dry run
	if opts.DryRun {
		r.ExecutedCount = 0
		r.StateAfter = buildState(configHash, finalURL, current)
		return r
	}

	// Step 11: top-level checkout if needed
	if !svn.IsWorkingCopy(opts.Workdir) {
		if err := svn.Checkout(ctx, opts.Client, opts.Workdir, finalURL, opts.Revision); err != nil {
			r.Err = fmt.Errorf("checkout: %w", err)
			return r
		}
	}

	// Step 12: execute actions
	executedCount := 0
	for i := range actions {
		a := &actions[i]
		var execErr error
		switch a.Kind {
		case plan.ActionAdd, plan.ActionUpgrade, plan.ActionDowngrade:
			execErr = svn.SetDepth(ctx, opts.Client, opts.Workdir, a.Path, a.ToDepth, opts.Revision)
		case plan.ActionExclude:
			execErr = svn.Exclude(ctx, opts.Client, opts.Workdir, a.Path, opts.Revision)
		}
		if execErr != nil {
			r.FailedAction = a
			r.Err = fmt.Errorf("%w: action %s %s: %w", ErrSvnFailed, a.Kind, a.Path, execErr)
			r.ExecutedCount = executedCount
			// Write half-state
			halfState := buildState("", finalURL, current)
			if saveErr := state.Save(opts.Workdir, halfState); saveErr != nil {
				r.Err = fmt.Errorf("%w; save state: %v", r.Err, saveErr)
			}
			r.StateAfter = halfState
			return r
		}
		executedCount++
		// Apply change to current map
		switch a.Kind {
		case plan.ActionAdd, plan.ActionUpgrade, plan.ActionDowngrade:
			current[a.Path] = a.ToDepth
		case plan.ActionExclude:
			delete(current, a.Path)
		}
	}

	r.ExecutedCount = executedCount

	// Step 13: write state on success
	newState := buildState(configHash, finalURL, current)
	if saveErr := state.Save(opts.Workdir, newState); saveErr != nil {
		r.Err = fmt.Errorf("save state: %w", saveErr)
	}
	r.StateAfter = newState
	return r
}

// Compute calculates the plan without executing svn commands or writing state.
func Compute(ctx context.Context, opts Options) (*Result, error) {
	opts.DryRun = true
	r := Apply(ctx, opts)
	if r.Err != nil {
		return nil, r.Err
	}
	return r, nil
}

func buildState(configHash, url string, current map[string]config.Depth) *state.State {
	paths := make([]state.PathEntry, 0, len(current))
	for p, d := range current {
		paths = append(paths, state.PathEntry{Path: p, Depth: d})
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].Path < paths[j].Path })
	return &state.State{
		Version:    state.StateVersion,
		ConfigHash: configHash,
		URL:        url,
		AppliedAt:  time.Now().UTC(),
		Paths:      paths,
	}
}
