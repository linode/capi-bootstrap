package cmd

import (
	"capi-bootstrap/providers"
	"capi-bootstrap/providers/infrastructure"
	"errors"

	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("please specify a cluster name")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDeleteCluster(cmd, args[0])
	},
}

func init() {
	deleteCmd.Flags().BoolP("force", "f", false,
		"delete all resources created by this cluster without confirming")
	rootCmd.AddCommand(deleteCmd)
}

func runDeleteCluster(cmd *cobra.Command, clusterName string) error {
	ctx := cmd.Context()
	var values providers.Values
	values.ClusterName = clusterName
	infrastructureProvider := infrastructure.NewInfrastructureProvider("LinodeCluster")
	err := infrastructureProvider.PreCmd(ctx, &values)
	if err != nil {
		return err
	}

	return infrastructureProvider.Delete(ctx, &values, cmd.Flags().Changed("force"))
}
