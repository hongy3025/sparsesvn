package plan

import (
	"fmt"

	"github.com/hongy3025/sparsesvn/internal/config"
)

type ActionKind int

const (
	ActionAdd ActionKind = iota
	ActionUpgrade
	ActionDowngrade
	ActionExclude
)

func (k ActionKind) String() string {
	switch k {
	case ActionAdd:
		return "add"
	case ActionUpgrade:
		return "upgrade"
	case ActionDowngrade:
		return "downgrade"
	case ActionExclude:
		return "exclude"
	default:
		return fmt.Sprintf("unknown(%d)", int(k))
	}
}

type Action struct {
	Kind      ActionKind
	Path      string
	FromDepth config.Depth
	ToDepth   config.Depth
}
