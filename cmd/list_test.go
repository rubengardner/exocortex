package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

func TestRunList_Empty(t *testing.T) {
	reg := &fakeRegistry{}
	out := &strings.Builder{}

	err := executeList(reg, out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "no agents") {
		t.Fatalf("expected empty state message, got: %s", out.String())
	}
}

func TestRunList_ShowsAgents(t *testing.T) {
	reg := &fakeRegistry{
		agents: []registry.Agent{
			{
				ID:              "fixaut",
				Branch:          "feat/fixaut",
				TaskDescription: "Fix auth bug",
				Status:          "idle",
				TmuxTarget:      "main:1.0",
				CreatedAt:       time.Now(),
			},
		},
	}
	out := &strings.Builder{}
	err := executeList(reg, out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := out.String()
	for _, want := range []string{"fixaut", "feat/fixaut", "Fix auth bug", "idle", "main:1.0"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected output to contain %q\ngot: %s", want, output)
		}
	}
}
