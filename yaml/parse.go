package yaml

import (
	"gopkg.in/yaml.v3"
)

type ControlPlane struct {
	Version  string `yaml:"apiVersion"`
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		InfrastructureTemplate struct {
			Kind string `yaml:"kind"`
			Name string `yaml:"name"`
		} `yaml:"infrastructureTemplate"`
		Version string `yaml:"version"`
	} `yaml:"spec"`
}

type ClusterType struct {
	Version  string `yaml:"apiVersion"`
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		ControlPlaneRef struct {
			Kind string `yaml:"kind"`
		} `yaml:"controlPlaneRef"`
	} `yaml:"spec"`
}
type LinodeMachineTemplate struct {
	Version  string `yaml:"apiVersion"`
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Region        string   `yaml:"region"`
				Type          string   `yaml:"type"`
				Image         string   `yaml:"image"`
				AuthorizeKeys []string `yaml:"authorizeKeys"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

func GetMachineDef(dec *yaml.Decoder, machineTemplateType string) *LinodeMachineTemplate {
	var template LinodeMachineTemplate
	for dec.Decode(&template) == nil {
		if template.Kind == machineTemplateType {
			return &template
		}
	}
	return nil
}

func GetControlPlaneDef(dec *yaml.Decoder, cpType string) *ControlPlane {
	var cp ControlPlane
	for dec.Decode(&cp) == nil {
		if cp.Kind == cpType {
			return &cp
		}
	}
	return nil
}

func GetClusterDef(dec *yaml.Decoder) *ClusterType {
	var cluster ClusterType
	for dec.Decode(&cluster) == nil {
		if cluster.Spec.ControlPlaneRef.Kind != "" {
			return &cluster
		}
	}
	return nil
}
