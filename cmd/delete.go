package cmd

import (
	"capi-bootstrap/pkg"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/linode/linodego"
	"github.com/spf13/cobra"
	"k8s.io/utils/ptr"
	"log"
	"os"
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
		log.Fatal("linode_token is required")
	}

	client := pkg.LinodeClient(linodeToken, ctx)
	ListFilter, err := json.Marshal(map[string]string{"tags": clusterName})
	if err != nil {
		log.Fatal(err)
	}

	instances, err := client.ListInstances(ctx, ptr.To(linodego.ListOptions{
		Filter: string(ListFilter),
	}))
	if err != nil {
		log.Fatalf("Could not list instances: %v", err)
	}
	if len(instances) > 0 {
		cmd.Print("Deleting instances:\n")
		for _, instance := range instances {
			cmd.Printf("  Label: %s, ID: %d\n", instance.Label, instance.ID)
		}
	}
	nodeBal, err := client.ListNodeBalancers(ctx, linodego.NewListOptions(1, string(ListFilter)))
	if err != nil {
		log.Fatalf("failed to list existing NodeBalancers: %v", err)
		return err
	}
	switch len(nodeBal) {
	case 1:
		cmd.Print("Deleting NodeBalancer:\n")
		cmd.Printf("  Label: %s, ID: %d\n", *nodeBal[0].Label, nodeBal[0].ID)

	case 0:
		cmd.Println("No NodeBalancers found for deletion")
	default:
		log.Fatalf("More than one NodeBalaner found for deletion, cannot delete")
	}
	cmd.Print("Would you like to delete these resources(y/n): ")
	var confirm string
	if !cmd.Flags().Changed("force") {
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "yes" {
			return nil
		}
	}

	cmd.Println("Deleting resources:")

	for _, instance := range instances {
		if err := client.DeleteInstance(ctx, instance.ID); err != nil {
			log.Fatalf("Could not delete instance %s: %v", instance.Label, err)
		}
		cmd.Printf("  Deleted Instance %s\n", instance.Label)
	}

	if len(nodeBal) == 1 {
		if err := client.DeleteNodeBalancer(ctx, nodeBal[0].ID); err != nil {
			log.Fatalf("Could not delete instance %s: %v", *nodeBal[0].Label, err)
		}
		cmd.Printf("  Deleted NodeBalancer %s\n", *nodeBal[0].Label)
	}

	return nil
}
