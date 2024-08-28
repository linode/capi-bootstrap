package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	"capi-bootstrap/types"
)

const (
	AppName = "capi-bootstrap"
)

var (
	configFile        string
	profile           string
	configFileDefault = filepath.Join("$XDG_CONFIG_HOME", "cluster-api", "bootstrap.yaml")
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:               AppName,
	Short:             "",
	Long:              ``,
	PersistentPreRunE: loadConfig,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(version, commit, date string) {
	appVersion = fmt.Sprintf("%s - %s %s %s", AppName, version, commit, date)
	err := rootCmd.Execute()
	if err != nil {
		klog.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", configFileDefault, "config file (default is "+configFileDefault+")")
	rootCmd.PersistentFlags().StringVar(&clusterOpts.backend, "backend", "",
		"The backend provider to use with "+AppName)

	rootCmd.PersistentFlags().StringVar(&profile, "profile", "",
		"Which profile to use from the configuration file")

	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlag(flag.CommandLine.Lookup("v"))
}

func loadConfig(cmd *cobra.Command, args []string) error {
	configFile = strings.Replace(configFile, "$XDG_CONFIG_HOME", os.Getenv("XDG_CONFIG_HOME"), -1)

	klog.V(5).Infof("looking for config file: %s", configFile)
	_, err := os.Stat(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(5).Infof("config file not found: %s", configFile)
			return nil //lint:ignore nilerr reason no config file
		}
		return err
	}

	// assume there is a config file
	c, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}

	var config types.Config
	if err := yaml.Unmarshal(c, &config); err != nil {
		return err
	}

	// precedence: arg > profile > default
	defaults := config.Defaults
	if profile != "" {
		// profile passed, try and find it
		if p, ok := config.Profiles[profile]; ok {
			klog.V(5).Infof("configuration profile found: %s", profile)
			defaults = p
		}
	}

	// expand in alpha order: backend, capi, controlPlane, infrastructure
	if err := expand(clusterOpts.backend, defaults.Backend, config.Backend); err != nil {
		return err
	}

	if err := expand(clusterOpts.capi, defaults.CAPI, config.CAPI); err != nil {
		return err
	}

	if err := expand(clusterOpts.controlPlane, defaults.ControlPlane, config.ControlPlane); err != nil {
		return err
	}

	if err := expand(clusterOpts.infrastructure, defaults.Infrastructure, config.Infrastructure); err != nil {
		return err
	}

	return nil
}

func expand(arg string, def string, envs map[string]types.Env) error {
	if arg != "" {
		// arg was passed, try to expand env
		klog.V(5).Infof("looking for env for argument: %s", arg)

		if e, ok := envs[arg]; ok {
			klog.V(5).Infof("env for argument found, expanding: %s", arg)
			if err := e.Expand(); err != nil {
				return err
			}
		}
	} else {
		// assume default/profile gets expanded
		klog.V(5).Infof("looking for env for: %s", def)
		for k, e := range envs {
			if k == def {
				klog.V(5).Infof("env found, expanding: %s", def)
				if err := e.Expand(); err != nil {
					return err
				}
				continue
			}
		}
	}

	return nil
}
