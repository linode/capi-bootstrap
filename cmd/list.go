package cmd

import (
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"capi-bootstrap/providers/backend"
	"capi-bootstrap/utils"
)

// listCmd represents the list command.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list clusters",
	Long:  `list clusters and nodes using config stored in a backend provider`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			args = []string{"main"}
			klog.Warningf("a branch name was not supplied via args, defaulting to main")
		}
		err := runListCluster(cmd, args[0])
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runListCluster(cmd *cobra.Command, branchName string) error {
	ctx := cmd.Context()

	backendProvider := backend.NewProvider(clusterOpts.backend)
	if err := backendProvider.PreCmd(ctx, branchName); err != nil {
		return err
	}

	clusters, err := backendProvider.ListClusters(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)

	err = utils.TabWriteClusters(w, clusters)
	if err != nil {
		return err
	}
	return w.Flush()
}
