package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

var putCmd = &cobra.Command{
	Use:   "put",
	Short: "",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		klog.Info("put called")
	},
}

func init() {
	rootCmd.AddCommand(putCmd)
}
