package cmd

import (
	"capi-bootstrap/client"
	"capi-bootstrap/cloudInit"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/drone/envsubst"
	"github.com/linode/linodego"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
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

	clusterCmd.Flags().StringVarP(&clusterOpts.manifest, "manifest", "m", "",
		"The file containing cluster manifest to use for bootstrap cluster")

	clusterCmd.Flags().StringVar(&clusterOpts.kubernetesVersion, "kubernetes-version", "",
		"The Kubernetes version to use for the workload cluster. If unspecified, the value from OS environment variables or the $XDG_CONFIG_HOME/cluster-api/clusterctl.yaml config file will be used.")

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
	var clusterName string
	if os.Getenv("CLUSTER_NAME") != "" {
		clusterName = os.Getenv("CLUSTER_NAME")
	}
	if len(args) != 0 {
		clusterName = args[0]
	}
	if clusterName == "" {
		return errors.New("cluster name is required")
	}
	klog.Infof("cluster name: %s", clusterName)
	// Define command-line flags for the input variables
	linodeToken := os.Getenv("LINODE_TOKEN")
	authorizedKeys := os.Getenv("AUTHORIZED_KEYS")
	region := os.Getenv("LINODE_REGION")

	if linodeToken == "" {
		return errors.New("linode_token is required")
	}

	linclient := client.LinodeClient(linodeToken, ctx)
	nbListFilter, err := json.Marshal(map[string]string{"tags": clusterName})
	if err != nil {
		return err
	}
	existingNB, err := linclient.ListNodeBalancers(ctx, linodego.NewListOptions(1, string(nbListFilter)))
	if err != nil {
		return err
	}
	if len(existingNB) != 0 {
		return err
	}
	// Create a NodeBalancer
	nodeBalancer, err := linclient.CreateNodeBalancer(ctx, linodego.NodeBalancerCreateOptions{
		Label:  &clusterName,
		Region: region,
		Tags:   []string{clusterName},
	})
	if err != nil {
		return err
	}
	klog.Infof("Created NodeBalancer: %v\n", *nodeBalancer.Label)

	// Create a NodeBalancer Config
	nodeBalancerConfig, err := linclient.CreateNodeBalancerConfig(ctx, nodeBalancer.ID, linodego.NodeBalancerConfigCreateOptions{
		Port:      6443,
		Protocol:  "tcp",
		Algorithm: "roundrobin",
		Check:     "connection",
	})
	if err != nil {
		return err
	}

	config, err := cloudInit.GenerateCloudInit(clusterName)
	if err != nil {
		return err
	}

	sub := map[string]string{
		"LINODE_TOKEN":    linodeToken,
		"AUTHORIZED_KEYS": "['" + authorizedKeys + "']",
		"NB_IP":           *nodeBalancer.IPv4,
		"NB_ID":           strconv.Itoa(nodeBalancer.ID),
		"NB_CONFIG_ID":    strconv.Itoa(nodeBalancerConfig.ID),
		"NB_PORT":         strconv.Itoa(nodeBalancerConfig.Port),
	}
	cloudConfig, err := envsubst.Eval(config, func(s string) string {
		return sub[s]
	})
	if err != nil {
		return err
	}

	// Create a Linode Instance
	instance, err := linclient.CreateInstance(ctx, linodego.InstanceCreateOptions{
		Label:          clusterName + "-bootstrap",
		Image:          "linode/debian11",
		Region:         region,
		Type:           "g6-standard-6",
		AuthorizedKeys: []string{authorizedKeys},
		RootPass:       "AkamaiPass123@1",
		Tags:           []string{clusterName},
		PrivateIP:      true,
		Metadata:       &linodego.InstanceMetadataOptions{UserData: base64.StdEncoding.EncodeToString([]byte(cloudConfig))},
	})
	if err != nil {
		return err
	}

	klog.Infof("Created Linode Instance: %v\n", instance.Label)

	var privateIP string

	for _, ip := range instance.IPv4 {
		if ip.IsPrivate() {
			privateIP = ip.String()
		}
	}

	// Create a NodeBalancer Node
	node, err := linclient.CreateNodeBalancerNode(ctx, nodeBalancer.ID, nodeBalancerConfig.ID, linodego.NodeBalancerNodeCreateOptions{
		Address: fmt.Sprintf("%s:6443", privateIP),
		Label:   clusterName + "-bootstrap",
		Weight:  100,
	})
	if err != nil {
		return err
	}

	klog.Infof("Created NodeBalancer Node: %v\n", node.Label)
	klog.Infof("Bootstrap Node IP: %s\n", instance.IPv4[0].String())
	return nil
}
