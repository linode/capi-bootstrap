package cmd

import (
	"github.com/spf13/cobra"
)

var getStateCmd = &cobra.Command{
	Use:   "state",
	Short: "get the state file for a cluster",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGetState(cmd, args[0])
	},
	Args: func(_ *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	getStateCmd.Flags().StringP("backend", "b", "",
		"backend to use for retrieving the kubeconfig")
	getCmd.AddCommand(getStateCmd)
}

func runGetState(cmd *cobra.Command, clusterName string) error {
	return nil
}
