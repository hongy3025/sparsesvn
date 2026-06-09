package logx

import (
	"encoding/json"
	"fmt"
	"io"
)

// Level controls the verbosity of log output.
type Level int

const (
	LevelQuiet   Level = iota // only Errorf
	LevelNormal               // default: plan summary + short command results
	LevelVerbose              // -v: each svn command
	LevelDebug                // -vv: each svn command stdout/stderr
)

// Logger is a leveled logger with optional JSON mode.
type Logger struct {
	out      io.Writer
	level    Level
	jsonMode bool
}

// New creates a Logger writing to out at the given level.
// When jsonMode is true, text methods are suppressed and JSON() outputs encoded JSON.
func New(out io.Writer, level Level, jsonMode bool) *Logger {
	return &Logger{out: out, level: level, jsonMode: jsonMode}
}

// Errorf logs an error message. Always outputs regardless of level.
func (l *Logger) Errorf(format string, args ...any) {
	if l.jsonMode {
		return
	}
	fmt.Fprintf(l.out, format+"\n", args...)
}

// Infof logs an informational message at LevelNormal and above.
func (l *Logger) Infof(format string, args ...any) {
	if l.jsonMode || l.level < LevelNormal {
		return
	}
	fmt.Fprintf(l.out, format+"\n", args...)
}

// Verbosef logs a verbose message at LevelVerbose and above.
func (l *Logger) Verbosef(format string, args ...any) {
	if l.jsonMode || l.level < LevelVerbose {
		return
	}
	fmt.Fprintf(l.out, format+"\n", args...)
}

// Debugf logs a debug message at LevelDebug only.
func (l *Logger) Debugf(format string, args ...any) {
	if l.jsonMode || l.level < LevelDebug {
		return
	}
	fmt.Fprintf(l.out, format+"\n", args...)
}

// JSON encodes v as JSON and writes it to the output.
// Only produces output when jsonMode is true.
func (l *Logger) JSON(v any) error {
	if !l.jsonMode {
		return nil
	}
	return json.NewEncoder(l.out).Encode(v)
}
