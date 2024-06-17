package yaml

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
	ApiServerPort        int
}
type Substitutions struct {
	ClusterName string
	K8sVersion  string
	Linode      LinodeSubstitutions
}

type ParsedManifest struct {
	ManifestFile    *InitFile
	AdditionalFiles []InitFile
	PreRunCmd       []string
	PostRunCmd      []string
}
