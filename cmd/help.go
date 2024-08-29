package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		klog.Info("help called")
		return nil
	},
}

func init() {
	helpCmd.Flags().StringP("backend", "b", "",
		"get help for a specific backend")
	helpCmd.Flags().StringP("control-plane", "c", "",
		"get help for a specific control plane")
	helpCmd.Flags().StringP("infrastructure", "i", "",
		"get help for a specific infrastructure")

	rootCmd.SetHelpCommand(helpCmd)
}
