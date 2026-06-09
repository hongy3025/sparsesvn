package svn

import "testing"

func TestParseVersion_OK(t *testing.T) {
	tests := []struct {
		input       string
		wantMajor   int
		wantMinor   int
		wantPatch   int
	}{
		{"1.14.2\n", 1, 14, 2},
		{"1.10.0", 1, 10, 0},
		{"1.14.2 (r1899510)\n...", 1, 14, 2},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotMajor, gotMinor, gotPatch, err := parseVersion(tt.input)
			if err != nil {
				t.Fatalf("parseVersion(%q) unexpected error: %v", tt.input, err)
			}
			if gotMajor != tt.wantMajor || gotMinor != tt.wantMinor || gotPatch != tt.wantPatch {
				t.Errorf("parseVersion(%q) = (%d,%d,%d), want (%d,%d,%d)",
					tt.input, gotMajor, gotMinor, gotPatch,
					tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
		})
	}
}

func TestParseVersion_Invalid(t *testing.T) {
	tests := []struct {
		input string
	}{
		{""},
		{"not a version"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, _, _, err := parseVersion(tt.input)
			if err == nil {
				t.Fatalf("parseVersion(%q) expected error, got nil", tt.input)
			}
		})
	}
}