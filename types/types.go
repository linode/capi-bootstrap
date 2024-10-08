package types

import (
	"io/fs"
	"os"

	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/klog/v2"
)

// Values is the struct including information parsed by all providers.
type Values struct {
	// ClusterName is the name of the cluster that is being deployed
	ClusterName string
	// BootstrapToken will pass a bootstrap token to a controlPlane provider, otherwise one will be generated
	BootstrapToken string
	// Namespace for resources to be installed into
	Namespace string
	// The generated Kubeconfig for a bootstrapped cluster
	Kubeconfig *v1.Config `json:"-"`
	// K8sVersion is the version parsed from the providers.ControlPlane
	K8sVersion string
	// SSHAuthorizedKeys will pass a list of ssh public keys to a controlplane provider
	SSHAuthorizedKeys []string
	// ClusterKind is the Kind of infrastructure.cluster.x-k8s.io used for this cluster
	ClusterKind string
	// ClusterEndpoint is the IP address or hostname to be used to access the kubernetes cluster
	ClusterEndpoint string
	// ManifestFile is the name of the file(or - for stdin) to read all manifests from
	ManifestFile string
	// ManifestFS is the local FS to read the ManifestFile from
	ManifestFS fs.FS `json:"-"`
	// BootstrapManifestDir sets the directory on the bootstrap machine for all generated k8s manifests written to
	BootstrapManifestDir string
	// Manifests is the separated list of all manifests parsed from the ManifestFile
	Manifests []string `json:"-"`
	// TarWriteFiles specifies whether a single tar files should be constructed for all write_files in order to deliver
	// reduce file sizes
	TarWriteFiles bool
}

type ClusterInfo struct {
	Name  string
	Nodes []*NodeInfo
}

type NodeInfo struct {
	Name              string
	Status            string
	Version           string
	ExternalIP        string
	DaysSinceCreation string
}

type Config struct {
	Defaults       Defaults
	Profiles       map[string]Defaults
	Backend        map[string]Env
	CAPI           map[string]Env
	ControlPlane   map[string]Env
	Infrastructure map[string]Env
}

type Defaults struct {
	Backend        string
	CAPI           string
	ControlPlane   string
	Infrastructure string
}

type Env map[string]string

func (e Env) Expand() error {
	for key, val := range e {
		klog.V(5).Infof("expanding env: %s=%s", key, val)
		if err := os.Setenv(key, val); err != nil {
			return err
		}
	}
	return nil
}
