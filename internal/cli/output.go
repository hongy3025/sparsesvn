package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/hongy3025/sparsesvn/internal/plan"
)

func kindMarker(k plan.ActionKind) string {
	switch k {
	case plan.ActionAdd:
		return "+"
	case plan.ActionUpgrade, plan.ActionDowngrade:
		return "~"
	case plan.ActionExclude:
		return "-"
	default:
		return "?"
	}
}

func kindLabel(k plan.ActionKind) string {
	switch k {
	case plan.ActionAdd:
		return "ADD"
	case plan.ActionUpgrade:
		return "UPGRADE"
	case plan.ActionDowngrade:
		return "DOWNGRADE"
	case plan.ActionExclude:
		return "EXCLUDE"
	default:
		return "UNKNOWN"
	}
}

func FormatPlan(actions []plan.Action) string {
	if len(actions) == 0 {
		return "Plan: 0 actions (no changes)"
	}

	var add, upgrade, downgrade, exclude int
	for _, a := range actions {
		switch a.Kind {
		case plan.ActionAdd:
			add++
		case plan.ActionUpgrade:
			upgrade++
		case plan.ActionDowngrade:
			downgrade++
		case plan.ActionExclude:
			exclude++
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Plan: %d actions (%d add, %d upgrade, %d downgrade, %d exclude)\n",
		len(actions), add, upgrade, downgrade, exclude)

	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, a := range actions {
		marker := kindMarker(a.Kind)
		label := kindLabel(a.Kind)
		switch a.Kind {
		case plan.ActionAdd:
			fmt.Fprintf(tw, "%s %s\t%s\t-> %s\n", marker, label, a.Path, a.ToDepth)
		case plan.ActionUpgrade, plan.ActionDowngrade:
			fmt.Fprintf(tw, "%s %s\t%s\t%s -> %s\n", marker, label, a.Path, a.FromDepth, a.ToDepth)
		case plan.ActionExclude:
			fmt.Fprintf(tw, "%s %s\t%s\t%s\n", marker, label, a.Path, a.FromDepth)
		}
	}
	tw.Flush()

	return strings.TrimRight(b.String(), "\n")
}

type PlanJSON struct {
	Url     string       `json:"url"`
	Actions []ActionJSON `json:"actions"`
	Summary SummaryJSON  `json:"summary"`
}

type ActionJSON struct {
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	FromDepth string `json:"from_depth,omitempty"`
	ToDepth   string `json:"to_depth,omitempty"`
}

type SummaryJSON struct {
	Add       int `json:"add"`
	Upgrade   int `json:"upgrade"`
	Downgrade int `json:"downgrade"`
	Exclude   int `json:"exclude"`
	Total     int `json:"total"`
}

func BuildPlanJSON(url string, actions []plan.Action) PlanJSON {
	pj := PlanJSON{
		Url:     url,
		Actions: make([]ActionJSON, 0, len(actions)),
	}

	var add, upgrade, downgrade, exclude int
	for _, a := range actions {
		aj := ActionJSON{
			Kind: kindLabel(a.Kind),
			Path: a.Path,
		}
		switch a.Kind {
		case plan.ActionAdd:
			aj.ToDepth = a.ToDepth.String()
			add++
		case plan.ActionUpgrade, plan.ActionDowngrade:
			aj.FromDepth = a.FromDepth.String()
			aj.ToDepth = a.ToDepth.String()
			if a.Kind == plan.ActionUpgrade {
				upgrade++
			} else {
				downgrade++
			}
		case plan.ActionExclude:
			aj.FromDepth = a.FromDepth.String()
			exclude++
		}
		pj.Actions = append(pj.Actions, aj)
	}

	pj.Summary = SummaryJSON{
		Add:       add,
		Upgrade:   upgrade,
		Downgrade: downgrade,
		Exclude:   exclude,
		Total:     len(actions),
	}

	return pj
}
