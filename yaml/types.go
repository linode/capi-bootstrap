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

type ParsedManifest struct {
	ManifestFile    *InitFile
	AdditionalFiles []InitFile
	PreRunCmd       []string
	PostRunCmd      []string
}
