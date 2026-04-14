package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/ruben_gardner/exocortex/internal/registry"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active nuclei",
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	reg := &registryAdapter{path: registry.DefaultPath()}
	return executeList(reg, cmd.OutOrStdout())
}

func executeList(reg nucleusSvc, out io.Writer) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	if len(r.Nuclei) == 0 {
		fmt.Fprintln(out, "no nuclei — run `exocortex new --task <description>` to create one")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tBRANCH\tTASK\tSTATUS\tNEURONS\tPRIMARY PANE")
	fmt.Fprintln(w, "--\t------\t----\t------\t-------\t------------")
	for _, n := range r.Nuclei {
		pane := "-"
		if p := n.PrimaryNeuron(); p != nil {
			pane = p.TmuxTarget
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
			n.ID, n.Branch, n.TaskDescription, n.Status, len(n.Neurons), pane)
	}
	return w.Flush()
}
