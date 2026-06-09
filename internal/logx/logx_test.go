package logx

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLevels(t *testing.T) {
	cases := []struct {
		name    string
		level   Level
		method  string
		wantOut bool
	}{
		// LevelQuiet: only Errorf outputs
		{"quiet/Errorf", LevelQuiet, "Errorf", true},
		{"quiet/Infof", LevelQuiet, "Infof", false},
		{"quiet/Verbosef", LevelQuiet, "Verbosef", false},
		{"quiet/Debugf", LevelQuiet, "Debugf", false},

		// LevelNormal: Errorf + Infof
		{"normal/Errorf", LevelNormal, "Errorf", true},
		{"normal/Infof", LevelNormal, "Infof", true},
		{"normal/Verbosef", LevelNormal, "Verbosef", false},
		{"normal/Debugf", LevelNormal, "Debugf", false},

		// LevelVerbose: Errorf + Infof + Verbosef
		{"verbose/Errorf", LevelVerbose, "Errorf", true},
		{"verbose/Infof", LevelVerbose, "Infof", true},
		{"verbose/Verbosef", LevelVerbose, "Verbosef", true},
		{"verbose/Debugf", LevelVerbose, "Debugf", false},

		// LevelDebug: all output
		{"debug/Errorf", LevelDebug, "Errorf", true},
		{"debug/Infof", LevelDebug, "Infof", true},
		{"debug/Verbosef", LevelDebug, "Verbosef", true},
		{"debug/Debugf", LevelDebug, "Debugf", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			l := New(&buf, tc.level, false)

			switch tc.method {
			case "Errorf":
				l.Errorf("msg")
			case "Infof":
				l.Infof("msg")
			case "Verbosef":
				l.Verbosef("msg")
			case "Debugf":
				l.Debugf("msg")
			}

			if tc.wantOut && buf.Len() == 0 {
				t.Errorf("expected output, got empty")
			}
			if !tc.wantOut && buf.Len() > 0 {
				t.Errorf("expected no output, got %q", buf.String())
			}
		})
	}
}

func TestJSONMode_SuppressesText(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelNormal, true)

	l.Infof("hello %s", "world")

	if buf.Len() != 0 {
		t.Errorf("expected no output in json mode, got %q", buf.String())
	}
}

func TestJSONMode_EmitsJSON(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelNormal, true)

	v := map[string]string{"k": "v"}
	if err := l.JSON(v); err != nil {
		t.Fatalf("JSON() error: %v", err)
	}

	want := `{"k":"v"}` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("JSON output = %q, want %q", got, want)
	}

	var decoded map[string]string
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

func TestNormalMode_NoJSON(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelNormal, false)

	v := map[string]string{"k": "v"}
	if err := l.JSON(v); err != nil {
		t.Fatalf("JSON() error: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("expected no output when jsonMode=false, got %q", buf.String())
	}
}