package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/klog/v2"
	k8syaml "sigs.k8s.io/yaml"
)

func NewBackend() *Backend {
	return &Backend{
		Name:     "file",
		BasePath: filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "cluster-api", "bootstrap"),
	}
}

type Backend struct {
	Name     string
	BasePath string
}

func (b *Backend) PreCmd(_ context.Context, clusterName string) error {
	klog.V(4).Infof("[file backend] trying to validate existing state dir: %s", b.BasePath)
	path := filepath.Join(b.BasePath, clusterName)
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		// no base path, make it
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}

	return nil
}

func (b *Backend) Read(_ context.Context, clusterName string) (*v1.Config, error) {
	file := filepath.Join(b.BasePath, clusterName, "kubeconfig.yaml")
	klog.V(4).Infof("[file backend] trying to read state file: %s", file)
	_, err := os.Stat(file)
	if err != nil && os.IsNotExist(err) {
		return nil, fmt.Errorf("state file does not exist: %s", file)
	}

	state, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	js, err := k8syaml.YAMLToJSON(state)
	if err != nil {
		return nil, err
	}

	var config v1.Config
	if err := json.Unmarshal(js, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (b *Backend) Write(_ context.Context, clusterName string, config *v1.Config) error {
	file := filepath.Join(b.BasePath, clusterName, "kubeconfig.yaml")
	klog.V(4).Infof("[file backend] trying to write state file: %s", file)

	js, err := json.Marshal(config)
	if err != nil {
		return err
	}

	y, err := k8syaml.JSONToYAML(js)
	if err != nil {
		return err
	}

	return os.WriteFile(file, y, 0755)
}

func (b *Backend) Delete(_ context.Context, clusterName string) error {
	path := filepath.Join(b.BasePath, clusterName)
	klog.V(4).Infof("[file backend] trying to delete state files in: %s", path)

	return os.RemoveAll(path)
}
