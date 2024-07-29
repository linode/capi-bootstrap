package infrastructure

import Linode "capi-bootstrap/providers/infrastructure/linode"

func NewInfrastructureProvider(name string) Provider {
	switch name {
	case "LinodeCluster":
		return Linode.CapL{}
	default:
		return nil
	}
}
