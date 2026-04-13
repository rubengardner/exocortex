package tmux

import (
	"fmt"
	"os/exec"
	"strings"
)

// Runner abstracts shell execution so callers can inject a fake in tests.
type Runner interface {
	Run(cmd string, args ...string) (string, error)
}

// ExecRunner is the real Runner that calls os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).Output()
	if err != nil {
		return "", fmt.Errorf("tmux: %w", err)
	}
	return string(out), nil
}

// Tmux wraps tmux operations via an injectable Runner.
type Tmux struct {
	runner Runner
}

// New returns a Tmux using the provided runner.
// Pass tmux.ExecRunner{} for real execution.
func New(r Runner) *Tmux {
	return &Tmux{runner: r}
}

// NewWindow opens a new tmux window with the given name and working directory.
// The window is created in the background (-d) so the caller's current window
// stays focused. Returns the pane target in "session:window.pane" format.
func (t *Tmux) NewWindow(workdir, name string) (string, error) {
	out, err := t.runner.Run("tmux",
		"new-window",
		"-d",
		"-n", name,
		"-c", workdir,
		"-P", "-F", "#{session_name}:#{window_index}.#{pane_index}",
	)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// SelectPane focuses the tmux pane identified by target ("session:window.pane").
// It switches the window first so the target becomes visible, then selects the pane.
func (t *Tmux) SelectPane(target string) error {
	// Switch to the window so it becomes the active window in the client's view.
	if _, err := t.runner.Run("tmux", "select-window", "-t", target); err != nil {
		return err
	}
	_, err := t.runner.Run("tmux", "select-pane", "-t", target)
	return err
}

// KillPane destroys the tmux pane identified by target.
func (t *Tmux) KillPane(target string) error {
	_, err := t.runner.Run("tmux", "kill-pane", "-t", target)
	return err
}

// SendKeys sends keystrokes to the pane identified by target, followed by Enter.
func (t *Tmux) SendKeys(target, keys string) error {
	_, err := t.runner.Run("tmux", "send-keys", "-t", target, keys, "Enter")
	return err
}

// CurrentTarget returns the target of the currently focused pane in
// "session:window.pane" format. Requires TMUX to be set (i.e. running inside tmux).
func (t *Tmux) CurrentTarget() (string, error) {
	out, err := t.runner.Run("tmux", "display-message", "-p",
		"#{session_name}:#{window_index}.#{pane_index}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// CapturePane returns the current visible text content of the pane identified
// by target. Output is plain text with no ANSI escape sequences.
func (t *Tmux) CapturePane(target string) (string, error) {
	out, err := t.runner.Run("tmux", "capture-pane", "-p", "-t", target)
	if err != nil {
		return "", err
	}
	return out, nil
}

// WindowExists reports whether the window/pane identified by target is alive.
// It returns false (not an error) when the window is simply gone.
func (t *Tmux) WindowExists(target string) (bool, error) {
	_, err := t.runner.Run("tmux", "list-panes", "-t", target)
	if err != nil {
		return false, nil
	}
	return true, nil
}
