package cmd

import (
	"capi-bootstrap/client"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/linode/linodego"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("please specify a cluster name")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDeleteCluster(cmd, args[0])
	},
}

func init() {
	deleteCmd.Flags().BoolP("force", "f", false,
		"delete all resources created by this cluster without confirming")
	rootCmd.AddCommand(deleteCmd)
}

func runDeleteCluster(cmd *cobra.Command, clusterName string) error {
	ctx := context.Background()
	linodeToken := os.Getenv("LINODE_TOKEN")

	if linodeToken == "" {
		return errors.New("linode_token is required")
	}

	linclient := client.LinodeClient(linodeToken, ctx)
	ListFilter, err := json.Marshal(map[string]string{"tags": clusterName})
	if err != nil {
		return err
	}

	instances, err := linclient.ListInstances(ctx, ptr.To(linodego.ListOptions{
		Filter: string(ListFilter),
	}))

	if err != nil {
		return fmt.Errorf("Could not list instances: %v", err)
	}

	if len(instances) > 0 {
		klog.Info("Deleting instances:\n")
		for _, instance := range instances {
			klog.Infof("  Label: %s, ID: %d\n", instance.Label, instance.ID)
		}
	}
	nodeBal, err := linclient.ListNodeBalancers(ctx, linodego.NewListOptions(1, string(ListFilter)))
	if err != nil {
		return err
	}
	switch len(nodeBal) {
	case 1:
		klog.Infof("Deleting NodeBalancer:\n")
		klog.Infof("  Label: %s, ID: %d\n", *nodeBal[0].Label, nodeBal[0].ID)

	case 0:
		klog.Infof("No NodeBalancers found for deletion")
	default:
		klog.Fatalf("More than one NodeBalaner found for deletion, cannot delete")
	}
	klog.Info("Would you like to delete these resources(y/n): ")
	var confirm string
	if !cmd.Flags().Changed("force") {
		if _, err := fmt.Scanln(&confirm); err != nil {
			return errors.New("error trying to read user input")
		}
		if confirm != "y" && confirm != "yes" {
			return nil
		}
	}

	klog.Info("Deleting resources:")

	for _, instance := range instances {
		if err := linclient.DeleteInstance(ctx, instance.ID); err != nil {
			return fmt.Errorf("Could not delete instance %s: %v", instance.Label, err)
		}
		klog.Infof("  Deleted Instance %s\n", instance.Label)
	}

	if len(nodeBal) == 1 {
		if err := linclient.DeleteNodeBalancer(ctx, nodeBal[0].ID); err != nil {
			return fmt.Errorf("Could not delete instance %s: %v", *nodeBal[0].Label, err)
		}
		klog.Infof("  Deleted NodeBalancer %s\n", *nodeBal[0].Label)
	}

	return nil
}
