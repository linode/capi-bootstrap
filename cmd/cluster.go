package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/drone/envsubst"
	"github.com/linode/linodego"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"k8s.io/utils/ptr"
	"log"
	"os"
	"strconv"
)

// clusterCmd represents the cluster command
var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBootstrapCluster(cmd, args)
	},
}

type clusterOptions struct {
	flavor                 string
	infrastructureProvider string

	manifest                 string
	kubernetesVersion        string
	controlPlaneMachineCount int64
	workerMachineCount       int64

	url string
}

var clusterOpts = &clusterOptions{}

func init() {

	clusterCmd.Flags().StringVar(&clusterOpts.kubernetesVersion, "kubernetes-version", "",
		"The Kubernetes version to use for the workload cluster. If unspecified, the value from OS environment variables or the $XDG_CONFIG_HOME/cluster-api/clusterctl.yaml config file will be used.")
	clusterCmd.Flags().StringVarP(&clusterOpts.manifest, "manifest", "m", "",
		"The file containing cluster manifest to use for bootstrap cluster")
	clusterCmd.Flags().Int64Var(&clusterOpts.controlPlaneMachineCount, "control-plane-machine-count", 1,
		"The number of control plane machines for the workload cluster.")
	// Remove default from hard coded text if the default is ever changed from 0 since cobra would then add it
	clusterCmd.Flags().Int64Var(&clusterOpts.workerMachineCount, "worker-machine-count", 0,
		"The number of worker machines for the workload cluster. (default 0)")

	// flags for the repository source
	clusterCmd.Flags().StringVarP(&clusterOpts.infrastructureProvider, "infrastructure", "i", "",
		"The infrastructure provider to read the workload cluster template from. If unspecified, the default infrastructure provider will be used.")
	clusterCmd.Flags().StringVarP(&clusterOpts.flavor, "flavor", "f", "",
		"The workload cluster template variant to be used when reading from the infrastructure provider repository. If unspecified, the default cluster template will be used.")

	// flags for the url source
	clusterCmd.Flags().StringVar(&clusterOpts.url, "from", "",
		"The URL to read the workload cluster template from. If unspecified, the infrastructure provider repository URL will be used. If set to '-', the workload cluster template is read from stdin.")

	// flags for the config map source
	rootCmd.AddCommand(clusterCmd)

}

func runBootstrapCluster(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	clusterName := ""
	if len(args) != 0 {
		clusterName = args[0]
	}
	log.Printf("cluster name: %s", clusterName)
	// Define command-line flags for the input variables
	linodeToken := os.Getenv("LINODE_TOKEN")
	authorizedKeys := os.Getenv("AUTHORIZED_KEYS")
	region := os.Getenv("LINODE_REGION")

	if linodeToken == "" {
		log.Fatal("linode_token is required")
	}

	// Set up OAuth2 authentication for Linode API
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: linodeToken})
	oauth2Client := oauth2.NewClient(ctx, tokenSource)

	client := linodego.NewClient(oauth2Client)

	// Create a NodeBalancer
	nodeBalancer, err := client.CreateNodeBalancer(ctx, linodego.NodeBalancerCreateOptions{
		Label:  ptr.To("capi-bootstrap"),
		Region: region,
		Tags:   []string{"capi-bootstrap"},
	})
	if err != nil {
		log.Fatalf("Error creating NodeBalancer: %v", err)
		return err
	}
	fmt.Printf("Created NodeBalancer: %v\n", *nodeBalancer.Label)

	// Create a NodeBalancer Config
	nodeBalancerConfig, err := client.CreateNodeBalancerConfig(ctx, nodeBalancer.ID, linodego.NodeBalancerConfigCreateOptions{
		Port:      6443,
		Protocol:  "tcp",
		Algorithm: "roundrobin",
		Check:     "connection",
	})
	if err != nil {
		log.Fatalf("Error creating NodeBalancer Config: %v", err)
		return err
	}

	b, err := os.ReadFile("cloud-config.yaml")
	if err != nil {
		fmt.Print(err)
		return err
	}

	sub := map[string]string{"LINODE_TOKEN": linodeToken, "AUTHORIZED_KEYS": "['" + authorizedKeys + "']", "NB_IP": *nodeBalancer.IPv4, "NB_ID": strconv.Itoa(nodeBalancer.ID), "NB_CONFIG_ID": strconv.Itoa(nodeBalancerConfig.ID)}
	cloudInit, err := envsubst.Eval(string(b), func(s string) string {
		return sub[s]
	})
	if err != nil {
		log.Fatal(err)
	}
	// Create a Linode Instance
	instance, err := client.CreateInstance(ctx, linodego.InstanceCreateOptions{
		Label:          "capi-bootstrap",
		Image:          "linode/debian11",
		Region:         region,
		Type:           "g6-standard-6",
		AuthorizedKeys: []string{authorizedKeys},
		RootPass:       "AkamaiPass123@1",
		Tags:           []string{"capi-bootstrap"},
		PrivateIP:      true,
		Metadata:       &linodego.InstanceMetadataOptions{UserData: base64.StdEncoding.EncodeToString([]byte(cloudInit))},
	})
	if err != nil {
		log.Fatalf("Error creating Linode Instance: %v", err)
		return err
	}
	fmt.Printf("Created Linode Instance: %v\n", instance.Label)

	var privateIP string

	for _, ip := range instance.IPv4 {
		if ip.IsPrivate() {
			privateIP = ip.String()
		}
	}

	// Create a NodeBalancer Node
	node, err := client.CreateNodeBalancerNode(ctx, nodeBalancer.ID, nodeBalancerConfig.ID, linodego.NodeBalancerNodeCreateOptions{
		Address: fmt.Sprintf("%s:6443", privateIP),
		Label:   "test-k3s",
		Weight:  100,
	})
	if err != nil {
		log.Fatalf("Error creating NodeBalancer Node: %v", err)
		return err
	}
	fmt.Printf("Created NodeBalancer Node: %v\n", node.Label)

	fmt.Printf("SSH: ssh root@%s\n", instance.IPv4[0].String())
	fmt.Printf("API Server: https://%s:6443\n", privateIP)
	return nil
}
