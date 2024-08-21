package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"capi-bootstrap/providers/backend"
	"capi-bootstrap/yaml"
)

// kubeconfigCmd represents the kubeconfig command.
var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "get kubeconfig for a cluster",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGetKubeconfig(cmd, args[0])
	},
	Args: func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("please specify a cluster name")
		}
		return nil
	},
}

func init() {
	kubeconfigCmd.Flags().StringP("backend", "b", "",
		"backend to use for retrieving the kubeconfig")
	getCmd.AddCommand(kubeconfigCmd)
}

func runGetKubeconfig(cmd *cobra.Command, clusterName string) error {
	var kconf []byte
	backendName, err := cmd.Flags().GetString("backend")
	if err != nil {
		return err
	}
	backendProvider := backend.NewProvider(backendName)
	if backendProvider == nil {
		return errors.New("backend provider not specified, options are: " + strings.Join(backend.ListProviders(), ","))
	}
	if err := backendProvider.PreCmd(cmd.Context(), clusterName); err != nil {
		return err
	}

	config, err := backendProvider.Read(cmd.Context(), clusterName)
	if err != nil {
		return err
	}
	kconf, err = yaml.Marshal(config)
	if err != nil {
		return err
	}

	fmt.Println(string(kconf))
	return nil
}
