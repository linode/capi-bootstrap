package cmd

import (
	"github.com/spf13/cobra"
)

var putStateCmd = &cobra.Command{
	Use:   "state",
	Short: "get the state file for a cluster",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPutState(cmd, args[0])
	},
	Args: func(_ *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	putStateCmd.Flags().StringP("backend", "b", "",
		"backend to use for retrieving the kubeconfig")
	putCmd.AddCommand(putStateCmd)
}

func runPutState(cmd *cobra.Command, clusterName string) error {
	return nil
}
