package logging

import (
	"fmt"
	"sync"
)

// CaptureLogger is an in-memory Logger for tests. It records every message with
// its level and exposes them for assertions. ANSI codes are stripped so tests
// can match on plain text.
type CaptureLogger struct {
	mu      sync.Mutex
	Records []Record
}

// Record is a single captured log line.
type Record struct {
	Level   Level
	Message string
}

// NewCapture returns an empty CaptureLogger.
func NewCapture() *CaptureLogger { return &CaptureLogger{} }

func (c *CaptureLogger) add(l Level, format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Records = append(c.Records, Record{Level: l, Message: StripANSI(msg)})
}

func (c *CaptureLogger) Debug(format string, args ...any) { c.add(LevelDebug, format, args...) }
func (c *CaptureLogger) Info(format string, args ...any)  { c.add(LevelInfo, format, args...) }
func (c *CaptureLogger) Warn(format string, args ...any)  { c.add(LevelWarning, format, args...) }
func (c *CaptureLogger) Error(format string, args ...any) { c.add(LevelError, format, args...) }

// Buffer returns INFO+ messages joined by newlines, mimicking the real memory
// sink used for the email body.
func (c *CaptureLogger) Buffer() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := ""
	for _, r := range c.Records {
		if r.Level >= LevelInfo {
			out += r.Message + "\n"
		}
	}
	return out
}

func (c *CaptureLogger) Close() error { return nil }

// Messages returns all captured messages (any level) in order.
func (c *CaptureLogger) Messages() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.Records))
	for i, r := range c.Records {
		out[i] = r.Message
	}
	return out
}

// Contains reports whether any captured message contains substr.
func (c *CaptureLogger) Contains(substr string) bool {
	for _, m := range c.Messages() {
		if containsStr(m, substr) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
