package yaml

type InitFile struct {
	Path        string `yaml:"path"`
	Content     string `yaml:"content,omitempty"`
	Source      Source `yaml:"source,omitempty"`
	Owner       string `yaml:"owner,omitempty"`
	Permissions string `yaml:"permissions,omitempty"`
	Encoding    string `yaml:"encoding,omitempty"`
	Append      bool   `yaml:"append,omitempty"`
	Defer       bool   `yaml:"defer,omitempty"`
}

type Source struct {
	URI     string            `yaml:"uri"`
	Headers map[string]string `yaml:"headers,omitempty"`
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
