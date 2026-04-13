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
	Short: "List all active agents",
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	reg := &registryAdapter{path: registry.DefaultPath()}
	return executeList(reg, cmd.OutOrStdout())
}

func executeList(reg registrySvc, out io.Writer) error {
	r, err := reg.Load()
	if err != nil {
		return err
	}
	if len(r.Agents) == 0 {
		fmt.Fprintln(out, "no agents — run `exocortex new --task <description>` to create one")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tBRANCH\tTASK\tSTATUS\tTMUX TARGET")
	fmt.Fprintln(w, "--\t------\t----\t------\t-----------")
	for _, a := range r.Agents {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			a.ID, a.Branch, a.TaskDescription, a.Status, a.TmuxTarget)
	}
	return w.Flush()
}
