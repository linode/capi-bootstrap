package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"capi-bootstrap/providers/backend"
	"capi-bootstrap/state"
)

// deleteCmd represents the delete command.
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

	// flags for the backend provider
	deleteCmd.Flags().StringVar(&clusterOpts.backend, "backend", "file",
		"The backend provider to use to store configuration for the cluster")

	rootCmd.AddCommand(deleteCmd)
}

func runDeleteCluster(cmd *cobra.Command, clusterName string) error {
	ctx := cmd.Context()

	backendProvider := backend.NewProvider(clusterOpts.backend)
	if err := backendProvider.PreCmd(ctx, clusterName); err != nil {
		return err
	}

	config, err := backendProvider.Read(ctx, clusterName)
	if err != nil {
		return err
	}

	clusterState, err := state.NewState(config)
	if err != nil {
		return err
	}

	if err := clusterState.Infrastructure.PreCmd(ctx, clusterState.Values); err != nil {
		return err
	}

	if err := clusterState.Infrastructure.Delete(ctx, clusterState.Values, cmd.Flags().Changed("force")); err != nil {
		return err
	}

	return backendProvider.Delete(ctx, clusterName)
}
