package yaml

import (
	"strings"

	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	capl "github.com/linode/cluster-api-provider-linode/api/v1alpha1"

	k3s "github.com/k3s-io/cluster-api-k3s/controlplane/api/v1beta1"

	"sigs.k8s.io/yaml"
)

func GetMachineDef(manifests []string, machineTemplateType string) *capl.LinodeMachineTemplate {
	var template capl.LinodeMachineTemplate
	for _, manifest := range manifests {
		_ = yaml.Unmarshal([]byte(manifest), &template)
		if template.Kind == machineTemplateType {
			return &template
		}
	}
	return nil
}

func GetControlPlaneDef(manifests []string, cpType string) *k3s.KThreesControlPlane {
	var cp k3s.KThreesControlPlane
	for _, manifest := range manifests {
		_ = yaml.Unmarshal([]byte(manifest), &cp)
		if cp.Kind == cpType {
			return &cp
		}
	}
	return nil
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

func UpdateManifest(yamlManifest string, values Substitutions) ([]byte, *ParsedManifest, error) {
	manifests := strings.Split(yamlManifest, "---")

	if err := UpdateCluster(manifests); err != nil {
		return nil, nil, err
	}

	if err := UpdateLinodeCluster(manifests, values); err != nil {
		return nil, nil, err
	}

	controlPlaneManifests, err := ParseControlPlane(manifests)
	if err != nil {
		return nil, nil, err
	}

	yamlManifest = strings.Join(manifests, "---\n")
	return []byte(yamlManifest), controlPlaneManifests, nil
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

func UpdateLinodeCluster(manifests []string, values Substitutions) error {
	var LinodeClusterIndex int
	var LinodeCluster capl.LinodeCluster
	for i, manifest := range manifests {
		err := yaml.Unmarshal([]byte(manifest), &LinodeCluster)
		if err != nil {
			return err
		}
		if LinodeCluster.Kind == "LinodeCluster" {
			LinodeCluster.Spec.ControlPlaneEndpoint = capi.APIEndpoint{
				Host: values.Linode.NodeBalancerIP,
				Port: int32(values.Linode.APIServerPort),
			}
			LinodeCluster.Spec.Network = capl.NetworkSpec{
				LoadBalancerType:     "NodeBalancer",
				LoadBalancerPort:     6443,
				NodeBalancerID:       &values.Linode.NodeBalancerID,
				NodeBalancerConfigID: &values.Linode.NodeBalancerConfigID,
			}
			LinodeClusterIndex = i
			break
		}
	}
	LinodeClusterString, err := yaml.Marshal(LinodeCluster)
	if err != nil {
		return err
	}
	manifests[LinodeClusterIndex] = string(LinodeClusterString)
	return nil
}

func ParseControlPlane(manifests []string) (*ParsedManifest, error) {
	var controlPlane k3s.KThreesControlPlane
	var controlPlaneManifests ParsedManifest
	for _, manifest := range manifests {
		err := yaml.Unmarshal([]byte(manifest), &controlPlane)
		if err != nil {
			return nil, err
		}
		if controlPlane.Kind == "KThreesControlPlane" {
			for _, file := range controlPlane.Spec.KThreesConfigSpec.Files {
				newFile := InitFile{
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
