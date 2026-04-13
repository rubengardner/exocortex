package tmux_test

import (
	"errors"
	"testing"

	"github.com/ruben_gardner/exocortex/internal/tmux"
)

type captureRunner struct {
	cmd    string
	args   []string
	output string
	err    error
}

func (r *captureRunner) Run(cmd string, args ...string) (string, error) {
	r.cmd = cmd
	r.args = args
	return r.output, r.err
}

func TestNewWindow_Args(t *testing.T) {
	r := &captureRunner{output: "main:1.2\n"}
	tm := tmux.New(r)

	target, err := tm.NewWindow("/some/worktree", "Fix auth bug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != "main:1.2" {
		t.Fatalf("expected trimmed target, got %q", target)
	}
	want := []string{
		"new-window",
		"-d",
		"-n", "Fix auth bug",
		"-c", "/some/worktree",
		"-P", "-F", "#{session_name}:#{window_index}.#{pane_index}",
	}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestNewWindow_PropagatesError(t *testing.T) {
	r := &captureRunner{err: errors.New("no session")}
	tm := tmux.New(r)

	_, err := tm.NewWindow("/path", "some task")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSelectPane_Args(t *testing.T) {
	r := &captureRunner{}
	tm := tmux.New(r)

	err := tm.SelectPane("main:1.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"select-pane", "-t", "main:1.2"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestKillPane_Args(t *testing.T) {
	r := &captureRunner{}
	tm := tmux.New(r)

	err := tm.KillPane("main:1.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"kill-pane", "-t", "main:1.2"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestSendKeys_Args(t *testing.T) {
	r := &captureRunner{}
	tm := tmux.New(r)

	err := tm.SendKeys("main:1.2", "nvim main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"send-keys", "-t", "main:1.2", "nvim main.go", "Enter"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestWindowExists_True(t *testing.T) {
	r := &captureRunner{} // err == nil → exit 0
	tm := tmux.New(r)

	exists, err := tm.WindowExists("main:1.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true when runner succeeds")
	}
}

func TestWindowExists_False(t *testing.T) {
	r := &captureRunner{err: errors.New("no such window")}
	tm := tmux.New(r)

	exists, err := tm.WindowExists("main:1.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatal("expected exists=false when runner fails")
	}
}

func TestCurrentTarget_Args(t *testing.T) {
	r := &captureRunner{output: "main:2.0\n"}
	tm := tmux.New(r)

	target, err := tm.CurrentTarget()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != "main:2.0" {
		t.Fatalf("expected trimmed target, got %q", target)
	}
	want := []string{"display-message", "-p", "#{session_name}:#{window_index}.#{pane_index}"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestCapturePane_Args(t *testing.T) {
	r := &captureRunner{output: "hello world\n"}
	tm := tmux.New(r)

	content, err := tm.CapturePane("main:1.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "hello world\n" {
		t.Fatalf("expected content, got %q", content)
	}
	want := []string{"capture-pane", "-p", "-t", "main:1.2"}
	if !equalSlice(r.args, want) {
		t.Fatalf("args mismatch\n got:  %v\n want: %v", r.args, want)
	}
}

func TestCapturePane_PropagatesError(t *testing.T) {
	r := &captureRunner{err: errors.New("no pane")}
	tm := tmux.New(r)

	_, err := tm.CapturePane("main:1.2")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCurrentTarget_PropagatesError(t *testing.T) {
	r := &captureRunner{err: errors.New("not in tmux")}
	tm := tmux.New(r)

	_, err := tm.CurrentTarget()
	if err == nil {
		t.Fatal("expected error")
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
