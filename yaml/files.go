package yaml

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"text/template"

	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/yaml"
)

func ConstructFile(filePath string, localPath string, filesystem fs.FS, values any, escapeYaml bool) (*InitFile, error) {
	manifest, err := templateManifest(filesystem, localPath, values, escapeYaml)
	if err != nil {
		return nil, err
	}
	initFile := InitFile{
		Path:    filePath,
		Content: string(manifest),
	}

	return &initFile, nil
}

func templateManifest(filesystem fs.FS, localPath string, templateValues any, escapeFile bool) ([]byte, error) {
	var err error
	tmpl := template.New(path.Base(localPath))
	tmpl.Delims("[[[", "]]]")
	rawYaml, err := fs.ReadFile(filesystem, localPath)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err)
	}
	escapedYaml := string(rawYaml)
	if escapeFile {
		// convert '{{ }}' to "{{ }}" then escape template
		escapedYaml = strings.ReplaceAll(string(rawYaml), "'{{", "\"{{")
		escapedYaml = strings.ReplaceAll(escapedYaml, "}}'", "}}\"")
		escapedYaml = strings.ReplaceAll(escapedYaml, "{{", "{{ '{{")
		escapedYaml = strings.ReplaceAll(escapedYaml, "}}", "}}' }}")
	}
	tmpl, err = tmpl.Parse(escapedYaml)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s, %s", localPath, err)
	}

	var b []byte
	buf := bytes.NewBuffer(b)
	err = tmpl.Execute(buf, templateValues)
	if err != nil {
		return nil, fmt.Errorf("failed to execute template %s, %s", localPath, err)
	}
	return buf.Bytes(), nil
}

func UpdateCluster(manifests []string) error {
	var cluster capi.Cluster
	var clusterIndex int
	for i, manifest := range manifests {
		err := yaml.Unmarshal([]byte(manifest), &cluster)
		if err != nil {
			return err
		}
		if cluster.Kind == "Cluster" {
			cluster.Spec.ControlPlaneRef.Name = "fake-control-plane"
			clusterIndex = i
			break
		}
	}
	clusterString, err := yaml.Marshal(cluster)
	if err != nil {
		return err
	}
	manifests[clusterIndex] = string(clusterString)
	return nil
}

// Marshal returns a marshaled yaml document based on the kubernetes library parsing.
func Marshal(obj interface{}) ([]byte, error) {
	return yaml.Marshal(obj)
}

func GetClusterDef(manifests []string) *capi.Cluster {
	var cluster capi.Cluster
	for _, manifest := range manifests {
		_ = yaml.Unmarshal([]byte(manifest), &cluster)
		if cluster.Kind == "Cluster" && cluster.APIVersion == "cluster.x-k8s.io/v1beta1" {
			return &cluster
		}
	}
	return nil
}
