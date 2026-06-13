package dump

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
)

// ErrBinaryNotFound is returned (wrapped) when the dump binary cannot be found
// or started. The caller treats this as fatal (exit 1), matching Python.
var ErrBinaryNotFound = errors.New("dump binary not found")

// CommandRunner runs an external command, streaming stdout to out and capturing
// stderr. A non-zero exit code is reported via exitCode, NOT via err; err is
// non-nil only when the process could not be started.
type CommandRunner interface {
	Run(name string, args []string, out io.Writer) (exitCode int, stderr string, err error)
}

// execRunner is the production CommandRunner backed by os/exec.
type execRunner struct{}

// DefaultRunner is the os/exec-backed runner used in production.
func DefaultRunner() CommandRunner { return execRunner{} }

func (execRunner) Run(name string, args []string, out io.Writer) (int, string, error) {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stdout = out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return 0, stderr.String(), nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// Ran, but exited non-zero. Not a start failure.
		return exitErr.ExitCode(), stderr.String(), nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return 0, stderr.String(), ErrBinaryNotFound
	}
	// Other start failures (permission, etc.) are also fatal for our purposes.
	return 0, stderr.String(), ErrBinaryNotFound
}
