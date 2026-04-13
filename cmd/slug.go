package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ruben_gardner/exocortex/internal/registry"
)

var nonAlpha = regexp.MustCompile(`[^a-z0-9]`)

// slugify converts a task description to a short, typeable ID.
// e.g. "Fix auth bug" → "fixau"
func slugify(task string) string {
	s := strings.ToLower(task)
	s = nonAlpha.ReplaceAllString(s, "")
	if len(s) > 6 {
		s = s[:6]
	}
	return s
}

// uniqueID returns a slug-based ID that does not collide with existing agent IDs.
// Appends a numeric suffix if needed.
func uniqueID(task string, agents []registry.Agent) string {
	base := slugify(task)
	existing := make(map[string]struct{}, len(agents))
	for _, a := range agents {
		existing[a.ID] = struct{}{}
	}
	if _, ok := existing[base]; !ok {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s%d", base, i)
		if _, ok := existing[candidate]; !ok {
			return candidate
		}
	}
}
