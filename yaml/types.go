package yaml

import "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta1"

type InitFile struct {
	Path        string `yaml:"path"`
	Content     string `yaml:"content"`
	Owner       string `yaml:"owner,omitempty"`
	Permissions string `yaml:"permissions,omitempty"`
	Encoding    string `yaml:"encoding,omitempty"`
}

type Config struct {
	WriteFiles []InitFile `yaml:"write_files"`
	RunCmd     []string   `yaml:"runcmd"`
}

type LinodeSubstitutions struct {
	Token                string
	AuthorizedKeys       string
	NodeBalancerIP       string
	NodeBalancerID       int
	NodeBalancerConfigID int
	APIServerPort        int
	VPC                  bool
}
type Substitutions struct {
	ClusterName string
	K8sVersion  string
	Linode      LinodeSubstitutions
	K3s         K3sSubstitutions
}

type K3sSubstitutions struct {
	ServerConfig v1beta1.KThreesServerConfig
	AgentConfig  v1beta1.KThreesAgentConfig
}

type ParsedManifest struct {
	ManifestFile    *InitFile
	AdditionalFiles []InitFile
	PreRunCmd       []string
	PostRunCmd      []string
}
