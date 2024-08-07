package cmd

import (
	"fmt"
	"os"

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
	appVersion = fmt.Sprintf("%s - %s %s %s", AppName, version, commit, date)
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $XDG_HOME_CONFIG/.capi-bootstrap.yaml)")
}
