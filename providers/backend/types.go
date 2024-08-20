package backend

import (
	"context"

	v1 "k8s.io/client-go/tools/clientcmd/api/v1"

	"capi-bootstrap/types"
	capiYaml "capi-bootstrap/yaml"
)

type Provider interface {
	PreCmd(ctx context.Context, clusterName string) error
	Read(ctx context.Context, clusterName string) (*v1.Config, error)
	WriteConfig(ctx context.Context, clusterName string, config *v1.Config) error
	WriteFiles(ctx context.Context, clusterName string, cloudInitFile *capiYaml.Config) ([]string, error)
	Delete(ctx context.Context, clusterName string) error
	ListClusters(context.Context) ([]types.ClusterInfo, error)
}
