package cmd

import (
	"os"

	"github.com/docker/distribution/context"

	"github.com/spf13/cobra"
)

const (
	AppName = "capi-bootstrap"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   AppName,
	Short: "",
	Long:  ``,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version, commit, date string) {
	ctx := context.WithValues(rootCmd.Context(), map[string]interface{}{
		"version": version,
		"commit":  commit,
		"date":    date,
	})
	rootCmd.SetContext(ctx)
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.capi-bootstrap.yaml)")
}
