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
		ctx := cmd.Context()
		klog.Infof("%s - %s %s %s",
			AppName, ctx.Value("version"), ctx.Value("commit"), ctx.Value("date"))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
