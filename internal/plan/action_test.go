package plan

import "testing"

func TestActionKindString(t *testing.T) {
	cases := []struct {
		kind ActionKind
		want string
	}{
		{ActionAdd, "add"},
		{ActionUpgrade, "upgrade"},
		{ActionDowngrade, "downgrade"},
		{ActionExclude, "exclude"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			got := c.kind.String()
			if got != c.want {
				t.Fatalf("ActionKind(%d).String() = %q, want %q", c.kind, got, c.want)
			}
		})
	}
}
