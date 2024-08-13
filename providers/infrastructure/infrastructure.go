package infrastructure

import "capi-bootstrap/providers/infrastructure/linode"

func NewProvider(name string) Provider {
	switch name {
	case "LinodeCluster":
		return linode.NewInfrastructure()
	default:
		return nil
	}
}
