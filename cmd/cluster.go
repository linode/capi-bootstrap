package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"capi-bootstrap/cloudinit"
	"capi-bootstrap/providers/backend"
	"capi-bootstrap/providers/controlplane"
	"capi-bootstrap/providers/infrastructure"
	"capi-bootstrap/state"
	"capi-bootstrap/types"
	capiYaml "capi-bootstrap/yaml"
)

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "",
	Long:  ``,
	RunE:  runBootstrapCluster,
}

type clusterOptions struct {
	backend        string
	capi           string
	controlPlane   string
	infrastructure string

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
	clusterCmd.Flags().StringVarP(&clusterOpts.infrastructure, "infrastructure", "i", "",
		"The infrastructure provider to read the workload cluster template from. If unspecified, the default infrastructure provider will be used.")
	clusterCmd.Flags().StringVarP(&clusterOpts.controlPlane, "controlplane", "c", "",
		"The control plane provider to use for this cluster.")
	clusterCmd.Flags().StringVarP(&clusterOpts.capi, "capi", "", "",
		"The CAPI provider configuration that should be used.")

	// flags for the url source
	clusterCmd.Flags().StringVar(&clusterOpts.url, "from", "",
		"The URL to read the workload cluster template from. If unspecified, the infrastructure provider repository URL will be used. If set to '-', the workload cluster template is read from stdin.")

	// flags for the config map source
	rootCmd.AddCommand(clusterCmd)
}

func runBootstrapCluster(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	manifestFile, err := cmd.Flags().GetString("manifest")
	if err != nil {
		return err
	}
	manifestFileName := filepath.Base(manifestFile)
	values := &types.Values{
		ManifestFile: manifestFileName,
	}
	if os.Getenv("AUTHORIZED_KEYS") != "" {
		keys := os.Getenv("AUTHORIZED_KEYS")
		values.SSHAuthorizedKeys = strings.Split(keys, ",")
		klog.V(4).Infof("using ssh public key(s) %s", values.SSHAuthorizedKeys)
	} else {
		klog.V(4).Infof("no ssh public key(s) were specified")
	}
	values.ManifestFS = os.DirFS(filepath.Dir(manifestFile))
	if manifestFileName == "-" {
		values.ManifestFS = cloudinit.IoFS{Reader: cmd.InOrStdin()}
	}

	capiManifests, err := cloudinit.GenerateCapiManifests(ctx, values, nil, nil, false)
	if err != nil {
		return fmt.Errorf("could not parse manifest %s: %s", values.ManifestFile, err)
	}

	values.Manifests = strings.Split(capiManifests.ManifestFile.Content, "---")

	clusterSpec := capiYaml.GetClusterDef(values.Manifests)
	if clusterSpec == nil {
		return errors.New("cluster not found")
	}
	values.Namespace = clusterSpec.Namespace

	infrastructureProvider := infrastructure.NewProvider(clusterSpec.Spec.InfrastructureRef.Kind)
	if infrastructureProvider == nil {
		return errors.New("infrastructure provider not found for " + clusterSpec.Spec.InfrastructureRef.Kind)
	}
	controlPlaneProvider := controlplane.NewProvider(clusterSpec.Spec.ControlPlaneRef.Kind)
	if controlPlaneProvider == nil {
		return errors.New("ControlPlane provider not found for " + clusterSpec.Spec.ControlPlaneRef.Kind)
	}
	values.ClusterName = clusterSpec.Name
	values.ClusterKind = clusterSpec.Spec.InfrastructureRef.Kind

	backendProvider := backend.NewProvider(clusterOpts.backend)
	if backendProvider == nil {
		return errors.New("backend provider not specified, options are: " + strings.Join(backend.ListProviders(), ", "))
	}
	if err := backendProvider.PreCmd(ctx, values.ClusterName); err != nil {
		return err
	}

	_, err = backendProvider.Read(ctx, values.ClusterName)
	if err == nil { // cluster already exists, don't overwrite it
		return errors.New("cluster state already exists in backend, delete before trying again")
	}

	if values.ClusterName == "" {
		return errors.New("cluster name is empty")
	}
	klog.Infof("cluster name: %s", values.ClusterName)

	if err := infrastructureProvider.PreCmd(ctx, values); err != nil {
		return err
	}

	if err := infrastructureProvider.PreDeploy(ctx, values); err != nil {
		return err
	}

	if err := controlPlaneProvider.PreDeploy(ctx, values); err != nil {
		return err
	}

	clusterState, err := state.NewState(values.Kubeconfig)
	if err != nil {
		return err
	}

	cloudConfig, err := cloudinit.GenerateCloudInit(ctx, values, infrastructureProvider, controlPlaneProvider, backendProvider)
	if err != nil {
		return err
	}

	if err := infrastructureProvider.Deploy(ctx, values, cloudConfig); err != nil {
		return err
	}

	if err := infrastructureProvider.PostDeploy(ctx, values); err != nil {
		return err
	}

	clusterState.Values = values
	clusterState.Backend = backendProvider
	clusterState.ControlPlane = controlPlaneProvider
	clusterState.Infrastructure = infrastructureProvider

	c, err := clusterState.ToConfig()
	if err != nil {
		return err
	}

	return backendProvider.WriteConfig(ctx, values.ClusterName, c)
}
