package controlplane

import (
	"capi-bootstrap/types"
	capiYaml "capi-bootstrap/yaml"
	"context"
)

type Provider interface {
	// GenerateCapiFile generates the CAPI controllers manifests needed to install an ControlPlaneProvider
	GenerateCapiFile(ctx context.Context, values *types.Values) (*capiYaml.InitFile, error)
	// GenerateInitScript generates init script needed to install an ControlPlaneProvider
	GenerateInitScript(ctx context.Context, initScriptPath string, values *types.Values) (*capiYaml.InitFile, error)
	// GenerateRunCommand generates run commands necessary to install a control plane ControlPlaneProvider
	GenerateRunCommand(ctx context.Context, values *types.Values) ([]string, error)
	// GenerateAdditionalFiles generates any additional manifests that might be necessary for the ControlPlaneProvider
	GenerateAdditionalFiles(ctx context.Context, values *types.Values) ([]capiYaml.InitFile, error)
	// UpdateManifests parses and updates any manifests needed to by the Provider
	UpdateManifests(ctx context.Context, manifests []string, values *types.Values) (*capiYaml.ParsedManifest, error)
	// PreDeploy takes in a common substitutions struct, does any setup needed to deploy a CAPI cluster and updates
	// the substitutions struct with any values needed by the ControlPlaneProvider
	PreDeploy(ctx context.Context, values *types.Values) error
}
