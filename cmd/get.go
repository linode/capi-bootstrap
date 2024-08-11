package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// getCmd represents the get command.
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		klog.Info("get called")
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
}
