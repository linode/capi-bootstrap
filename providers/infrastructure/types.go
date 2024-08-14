package infrastructure

import (
	"context"

	"capi-bootstrap/types"
	capiYaml "capi-bootstrap/yaml"
)

type Provider interface {
	// GenerateCapiFile generates the CAPI controllers manifests needed to install a Provider
	GenerateCapiFile(ctx context.Context, values *types.Values) (*capiYaml.InitFile, error)
	// GenerateCapiMachine generates manifests needed to adopt the current machine into CAPI
	GenerateCapiMachine(ctx context.Context, values *types.Values) (*capiYaml.InitFile, error)
	// GenerateAdditionalFiles generates any additional manifests that might be necessary for the Provider
	GenerateAdditionalFiles(ctx context.Context, values *types.Values) ([]capiYaml.InitFile, error)
	// UpdateManifests parses and updates any manifests needed to by the Provider
	UpdateManifests(ctx context.Context, manifests []string, values *types.Values) error
	// PreCmd does any validation an initial steps needed for doing any operations with a cluster in a Provider
	PreCmd(ctx context.Context, values *types.Values) error
	// PreDeploy takes in a common substitutions struct, does any setup needed to deploy a CAPI cluster and updates
	// the substitutions struct with any values needed by the Provider
	PreDeploy(ctx context.Context, values *types.Values) error
	// Deploy takes a common substitutions struct a []byte representation of the metadata required to bootstrap a cluster,
	// and it deploys any resources necessary for that bootstrapping with a specific Provider
	Deploy(ctx context.Context, values *types.Values, metadata []byte) error
	// PostDeploy takes a common substitutions struct and does any work necessary for bootstrapping with a specific Provider
	PostDeploy(ctx context.Context, values *types.Values) error
	// Delete a cluster for an associated InfrastructureProvider
	Delete(ctx context.Context, values *types.Values, force bool) error
}
