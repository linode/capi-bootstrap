package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// versionCmd
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the version of " + AppName,
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		klog.Info("version called")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
