package state

import (
	"capi-bootstrap/providers/backend"
	"capi-bootstrap/providers/controlplane"
	"capi-bootstrap/providers/infrastructure"
	"capi-bootstrap/types"
	"encoding/json"

	"k8s.io/apimachinery/pkg/runtime"

	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

const (
	ExtensionName = "capi-bootstrap"
)

type State struct {
	config         *v1.Config
	Values         *types.Values
	Infrastructure infrastructure.Provider
	Backend        backend.Provider
	ControlPlane   controlplane.Provider
}

func NewState(config *v1.Config) (*State, error) {
	s := &State{
		config: config,
	}

	for _, ext := range config.Extensions {
		if ext.Name == ExtensionName {
			if err := json.Unmarshal(ext.Extension.Raw, s); err != nil {
				return nil, err
			}
			break
		}
	}

	return s, nil
}

func (s *State) ToConfig() (*v1.Config, error) {
	config := s.config

	raw, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	// remove old extension if it exists
	removeExtension(config)

	// replace with current state contents
	config.Extensions = append(config.Extensions, v1.NamedExtension{
		Name: ExtensionName,
		Extension: runtime.RawExtension{
			Raw: raw,
		},
	})

	return config, nil
}

func (s *State) UnmarshalJSON(b []byte) error {
	raw := make(map[string]json.RawMessage)

	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	if v, ok := raw["Values"]; ok {
		if err := json.Unmarshal(v, &s.Values); err != nil {
			return err
		}
	}

	whoami := struct {
		Name string
	}{}

	if b, ok := raw["Backend"]; ok {
		if err := json.Unmarshal(b, &whoami); err != nil {
			return err
		}
		backendProvider := backend.NewProvider(whoami.Name)
		if err := json.Unmarshal(b, &backendProvider); err != nil {
			return err
		}
		s.Backend = backendProvider
	}

	if i, ok := raw["Infrastructure"]; ok {
		if err := json.Unmarshal(i, &whoami); err != nil {
			return err
		}
		infrastructureProvider := infrastructure.NewProvider(whoami.Name)
		if err := json.Unmarshal(i, &infrastructureProvider); err != nil {
			return err
		}
		s.Infrastructure = infrastructureProvider
	}

	if cp, ok := raw["ControlPlane"]; ok {
		if err := json.Unmarshal(cp, &whoami); err != nil {
			return err
		}
		controlplaneProvider := controlplane.NewProvider(whoami.Name)
		if err := json.Unmarshal(cp, &controlplaneProvider); err != nil {
			return err
		}
		s.ControlPlane = controlplaneProvider
	}

	return nil
}

func removeExtension(config *v1.Config) {
	for i, ext := range config.Extensions {
		if ext.Name == ExtensionName {
			config.Extensions = append(config.Extensions[:i], config.Extensions[i+1:]...)
			break
		}
	}
}
