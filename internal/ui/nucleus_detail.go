package ui

import (
	"fmt"
	"strings"
)

// viewNucleusDetail renders the right-panel detail for the selected nucleus.
func (m Model) viewNucleusDetail(width int) string {
	if len(m.nuclei) == 0 {
		return ""
	}
	n := m.nuclei[m.cursor]

	var sb strings.Builder

	// Compact header: id, task, branch, status.
	sb.WriteString(StyleTitle.Render(truncate(n.TaskDescription, width-4)) + "\n")
	sb.WriteString(StyleDim.Render(strings.Repeat("─", clamp(width-4, 4, 60))) + "\n")
	field := func(label, value string) string {
		return StyleLabel.Render(label) + StyleValue.Render(truncate(value, width-16)) + "\n"
	}
	sb.WriteString(field("ID", n.ID))
	sb.WriteString(field("Branch", n.Branch))
	sb.WriteString(StyleLabel.Render("Status") + StatusDot(n.Status) + " " + n.Status + "\n")
	primaryTarget := "—"
	if primary := n.PrimaryNeuron(); primary != nil {
		primaryTarget = primary.TmuxTarget
	}
	sb.WriteString(field("Claude", primaryTarget))
	nvimVal := "—"
	if nvim := n.NvimNeuron(); nvim != nil && nvim.TmuxTarget != "" {
		nvimVal = nvim.TmuxTarget
	}
	sb.WriteString(field("Nvim", nvimVal))

	headerLines := 7 // title + divider + id + branch + status + claude + nvim

	// Live preview section.
	previewHeaderLines := 2 // blank + separator
	previewLines := m.contentHeight() - headerLines - previewHeaderLines
	if previewLines < 1 {
		previewLines = 1
	}

	sb.WriteString("\n")
	previewLabel := "── Preview "
	if !m.previewEnabled {
		previewLabel = "── Preview [off] "
	}
	sb.WriteString(StyleDim.Render(previewLabel+strings.Repeat("─", clamp(width-len(previewLabel)-4, 4, 40))) + "\n")

	if !m.previewEnabled {
		sb.WriteString(StyleDim.Render("  press p to enable"))
	} else if m.services.CapturePane == nil {
		sb.WriteString(StyleDim.Render("  (preview not available)"))
	} else if m.paneContent == "" {
		sb.WriteString(StyleDim.Render("  loading…"))
	} else {
		lines := strings.Split(strings.TrimRight(m.paneContent, "\n"), "\n")
		// Strip trailing spaces from each line (capture-pane pads to pane width).
		for i, l := range lines {
			lines[i] = strings.TrimRight(l, " ")
		}
		// Trim trailing empty lines.
		for len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		// Show only the last N lines that fit.
		if len(lines) > previewLines {
			lines = lines[len(lines)-previewLines:]
		}
		for _, l := range lines {
			sb.WriteString(fmt.Sprintf("%s\n", truncate(l, width-2)))
		}
	}

	return sb.String()
}
