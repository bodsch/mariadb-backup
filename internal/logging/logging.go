// Package logging provides the three-sink logger used across the application,
// mirroring the behaviour of the original Python script's setup_logging:
//
//  1. a file sink (hardcoded /var/log/mariadb-backup.log) at the configured level,
//  2. a console sink (stdout) at INFO and above, with ANSI colours retained,
//  3. an in-memory buffer at INFO and above holding the raw messages, which is
//     used as the body of the notification email.
//
// If the log file is not writable (e.g. running as non-root) the file sink is
// disabled with a warning and the run continues — unlike the Python original,
// which crashed.
package logging

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
)

// Level is the logging level. The five names mirror the Python choices
// DEBUG/INFO/WARNING/ERROR/CRITICAL.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarning
	LevelError
	LevelCritical
)

// ANSI colour codes, mirroring the Python `bcolors` helper. They are written to
// the console sink and stripped from the email body.
const (
	ColorReset = "\033[0m"
	ColorDebug = "\033[1m\033[92m" // bold green
	ColorInfo  = "\033[94m"        // blue
	ColorWarn  = "\033[93m"        // yellow
	ColorFail  = "\033[91m"        // red
	ColorBold  = "\033[1m"
)

// DefaultLogFile is the hardcoded path of the file sink (matches Python).
const DefaultLogFile = "/var/log/mariadb-backup.log"

// ansiEscape matches ANSI escape sequences; identical to the regex used by the
// Python remove_ansi_escape_sequences helper.
var ansiEscape = regexp.MustCompile(`\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)

// StripANSI removes ANSI escape sequences from s.
func StripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// ParseLevel converts a level name (case-insensitive) to a Level. The bool is
// false if the name is unknown.
func ParseLevel(name string) (Level, bool) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "DEBUG":
		return LevelDebug, true
	case "INFO":
		return LevelInfo, true
	case "WARNING":
		return LevelWarning, true
	case "ERROR":
		return LevelError, true
	case "CRITICAL":
		return LevelCritical, true
	default:
		return LevelInfo, false
	}
}

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarning:
		return "WARNING"
	case LevelError:
		return "ERROR"
	case LevelCritical:
		return "CRITICAL"
	default:
		return "INFO"
	}
}

// Logger is the interface consumed by the rest of the application. Keeping it an
// interface lets tests inject a capturing fake.
type Logger interface {
	Debug(format string, args ...any)
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
	// Buffer returns the accumulated in-memory log (raw messages, INFO+),
	// used as the email body.
	Buffer() string
	// Close flushes and closes the file sink, if any.
	Close() error
}

// clock abstracts time formatting so tests stay deterministic.
type nowFunc func() (hms string, full string)

// logger is the concrete three-sink implementation.
type logger struct {
	mu     sync.Mutex
	level  Level
	file   *os.File // may be nil if the file sink is disabled
	buf    strings.Builder
	now    nowFunc
	stdout *os.File
}

// Options configures New.
type Options struct {
	Level Level
	// LogFile overrides the file sink path; empty uses DefaultLogFile.
	LogFile string
	// now is injectable for tests; nil uses the wall clock.
	now nowFunc
}

// New builds a three-sink logger. It never returns an error: if the file sink
// cannot be opened, it logs a warning to the console and continues without it.
func New(opts Options) Logger {
	path := opts.LogFile
	if path == "" {
		path = DefaultLogFile
	}
	now := opts.now
	if now == nil {
		now = wallClock
	}

	l := &logger{
		level:  opts.Level,
		now:    now,
		stdout: os.Stdout,
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		// Graceful degradation: warn and continue without the file sink.
		fmt.Fprintf(os.Stdout, "%sWARNING: log file %s not writable (%v); file logging disabled%s\n",
			ColorWarn, path, err, ColorReset)
	} else {
		l.file = f
	}

	return l
}

func wallClock() (string, string) {
	t := timeNow()
	return t.Format("15:04:05"), t.Format("2006-01-02 15:04:05")
}

func (l *logger) Buffer() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func (l *logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

func (l *logger) Debug(format string, args ...any) { l.log(LevelDebug, format, args...) }
func (l *logger) Info(format string, args ...any)  { l.log(LevelInfo, format, args...) }
func (l *logger) Warn(format string, args ...any)  { l.log(LevelWarning, format, args...) }
func (l *logger) Error(format string, args ...any) { l.log(LevelError, format, args...) }

func (l *logger) log(lvl Level, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	hms, full := l.now()

	// File sink: everything at or above the configured level. DEBUG uses the
	// short timestamp, matching the Python debug_formatter.
	if l.file != nil && lvl >= l.level {
		ts := full
		if l.level == LevelDebug {
			ts = hms
		}
		fmt.Fprintf(l.file, "%s - %s - %s\n", ts, lvl.String(), StripANSI(msg))
	}

	// Console and memory sinks: INFO and above.
	if lvl >= LevelInfo {
		fmt.Fprintf(l.stdout, "%s - %s - %s\n", full, lvl.String(), msg)
		// Memory buffer holds the raw message only (matches MemoryLogHandler).
		l.buf.WriteString(StripANSI(msg))
		l.buf.WriteByte('\n')
	}
}
