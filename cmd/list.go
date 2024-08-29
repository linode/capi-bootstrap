package cmd

import (
	"errors"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"capi-bootstrap/providers/backend"
	"capi-bootstrap/types"
	"capi-bootstrap/utils"
	capiYaml "capi-bootstrap/yaml"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list clusters",
	Long:  `list clusters and nodes using config stored in a backend provider`,
	RunE:  runListCluster,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runListCluster(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	backendProvider := backend.NewProvider(clusterOpts.backend)
	if backendProvider == nil {
		return errors.New("backend provider not specified, options are: " + strings.Join(backend.ListProviders(), ","))
	}
	if err := backendProvider.PreCmd(ctx, ""); err != nil {
		return err
	}

	clusterConfigs, err := backendProvider.ListClusters(ctx)
	if err != nil {
		return err
	}
	clusters := make([]types.ClusterInfo, 0, len(clusterConfigs))
	for name, conf := range clusterConfigs {
		kubeconfig, err := capiYaml.Marshal(conf)
		if err != nil {
			return err
		}
		list, err := utils.BuildNodeInfoList(ctx, kubeconfig)
		if err != nil {
			return err
		}
		clusters = append(clusters, types.ClusterInfo{
			Name:  name,
			Nodes: list,
		})
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)

	err = utils.TabWriteClusters(w, clusters)
	if err != nil {
		return err
	}
	return w.Flush()
}
