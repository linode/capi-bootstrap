package providers

import (
	"io/fs"

	"github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta1"
	"github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/linodego"
)

// Values is the struct including information parsed by all providers
type Values struct {
	// ClusterName is the name of the cluster that is being deployed
	ClusterName string
	// K8sVersion is the version parsed from the providers.ControlPlane
	K8sVersion string
	// ClusterKind is the Kind of infrastructure.cluster.x-k8s.io used for this cluster
	ClusterKind string
	// ClusterEndpoint is the IP address or hostname to be used to access the kubernetes cluster
	ClusterEndpoint string
	// ManifestFile is the name of the file(or - for stdin) to read all manifests from
	ManifestFile string
	// ManifestFS is the local FS to read the ManifestFile from
	ManifestFS fs.FS
	// BootstrapManifestDir sets the directory on the bootstrap machine for all generated k8s manifests written to
	BootstrapManifestDir string
	// Manifests is the separated list of all manifests parsed from the ManifestFile
	Manifests []string
	// Linode is a struct of all values needed by the Infrastructure.Linode provider
	Linode LinodeValues
	// K3s is a struct of all values needed by the ControlPlane.K3s provider
	K3s K3sValues
}

type LinodeValues struct {
	Client             linodego.Client
	Machine            *v1alpha1.LinodeMachineTemplate
	NodeBalancer       *linodego.NodeBalancer
	NodeBalancerConfig *linodego.NodeBalancerConfig
	Token              string
	AuthorizedKeys     string
	VPC                *v1alpha1.LinodeVPC
}

type K3sValues struct {
	ServerConfig v1beta1.KThreesServerConfig
	AgentConfig  v1beta1.KThreesAgentConfig
}
