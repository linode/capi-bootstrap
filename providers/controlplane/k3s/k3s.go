package k3s

import "C"
import (
	"capi-bootstrap/types"
	capiYaml "capi-bootstrap/yaml"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta1"
	capK3s "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta1"
	"github.com/k3s-io/cluster-api-k3s/pkg/etcd"
	"github.com/k3s-io/cluster-api-k3s/pkg/k3s"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

type ControlPlane struct {
	Name         string
	ServerConfig v1beta1.KThreesServerConfig
	AgentConfig  v1beta1.KThreesAgentConfig
}

func NewControlPlane() *ControlPlane {
	return &ControlPlane{
		Name: "KThreesControlPlane",
	}
}

func (p *ControlPlane) GenerateCapiFile(_ context.Context, values *types.Values) (*capiYaml.InitFile, error) {
	filePath := "/var/lib/rancher/k3s/server/manifests/capi-k3s.yaml"
	return capiYaml.ConstructFile(filePath, filepath.Join("files", "capi-k3s.yaml"), files, values, false)
}

func (p *ControlPlane) GenerateAdditionalFiles(_ context.Context, values *types.Values) ([]capiYaml.InitFile, error) {
	configFile, err := p.generateK3sConfig(values)
	if err != nil {
		return nil, err
	}
	manifestFile, err := generateK3sManifests()
	if err != nil {
		return nil, err
	}
	return []capiYaml.InitFile{
		*configFile, *manifestFile,
	}, nil
}

func (p *ControlPlane) PreDeploy(_ context.Context, values *types.Values) error {
	controlPlaneSpec := GetControlPlaneDef(values.Manifests)
	if controlPlaneSpec == nil {
		return errors.New("control plane not found")
	}

	p.ServerConfig = controlPlaneSpec.Spec.KThreesConfigSpec.ServerConfig
	p.AgentConfig = controlPlaneSpec.Spec.KThreesConfigSpec.AgentConfig

	values.BootstrapManifestDir = "/var/lib/rancher/k3s/server/manifests/"
	// set the k8s version as parsed from the ControlPlane
	values.K8sVersion = controlPlaneSpec.Spec.Version
	klog.Infof("k8s version : %s", controlPlaneSpec.Spec.Version)
	return nil
}

func (p *ControlPlane) GenerateRunCommand(_ context.Context, values *types.Values) ([]string, error) {
	return []string{fmt.Sprintf("curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=%q sh -s - server", values.K8sVersion)}, nil
}

func (p *ControlPlane) GenerateInitScript(_ context.Context, initScriptPath string, values *types.Values) (*capiYaml.InitFile, error) {
	return capiYaml.ConstructFile(initScriptPath, filepath.Join("files", "init-cluster.sh"), files, values, false)
}

func GetControlPlaneDef(manifests []string) *capK3s.KThreesControlPlane {
	var cp capK3s.KThreesControlPlane
	for _, manifest := range manifests {
		_ = yaml.Unmarshal([]byte(manifest), &cp)
		if cp.Kind == "KThreesControlPlane" {
			return &cp
		}
	}
	return nil
}

func (p *ControlPlane) generateK3sConfig(values *types.Values) (*capiYaml.InitFile, error) {
	filePath := "/etc/rancher/k3s/config.yaml"
	if values.BootstrapToken == "" {
		values.BootstrapToken = uuid.NewString()
	}
	config := k3s.GenerateInitControlPlaneConfig(values.ClusterEndpoint, values.BootstrapToken, p.ServerConfig, p.AgentConfig)
	configYaml, err := capiYaml.Marshal(config)
	if err != nil {
		return nil, err
	}
	return &capiYaml.InitFile{
		Path:    filePath,
		Content: string(configYaml),
	}, nil
}

func generateK3sManifests() (*capiYaml.InitFile, error) {
	return &capiYaml.InitFile{
		Path:    etcd.EtcdProxyDaemonsetYamlLocation,
		Content: etcd.EtcdProxyDaemonsetYaml,
	}, nil
}

func (p *ControlPlane) UpdateManifests(_ context.Context, manifests []string, values *types.Values) (*capiYaml.ParsedManifest, error) {
	var controlPlane capK3s.KThreesControlPlane
	var controlPlaneManifests capiYaml.ParsedManifest
	for _, manifest := range manifests {
		err := yaml.Unmarshal([]byte(manifest), &controlPlane)
		if err != nil {
			return nil, err
		}
		if controlPlane.Kind == "KThreesControlPlane" {
			for _, file := range controlPlane.Spec.KThreesConfigSpec.Files {
				newFile := capiYaml.InitFile{
					Path:        file.Path,
					Content:     file.Content,
					Owner:       file.Owner,
					Permissions: file.Permissions,
					Encoding:    string(file.Encoding),
				}
				controlPlaneManifests.AdditionalFiles = append(controlPlaneManifests.AdditionalFiles, newFile)
			}
			for _, cmd := range controlPlane.Spec.KThreesConfigSpec.PreK3sCommands {
				parsedCommand := strings.ReplaceAll(cmd, "{{ '{{", "{{")
				parsedCommand = strings.ReplaceAll(parsedCommand, "}}' }}", "}}")
				controlPlaneManifests.PreRunCmd = append(controlPlaneManifests.PreRunCmd, parsedCommand)
			}
			for _, cmd := range controlPlane.Spec.KThreesConfigSpec.PostK3sCommands {
				parsedCommand := strings.ReplaceAll(cmd, "{{ '{{", "{{")
				parsedCommand = strings.ReplaceAll(parsedCommand, "}}' }}", "}}")
				controlPlaneManifests.PostRunCmd = append(controlPlaneManifests.PostRunCmd, parsedCommand)
			}
		}

	}
	return &controlPlaneManifests, nil
}
