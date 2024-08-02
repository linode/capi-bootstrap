package infrastructure

import Linode "capi-bootstrap/providers/infrastructure/linode"

func NewProvider(name string) Provider {
	switch name {
	case "LinodeCluster":
		return Linode.CAPL{}
	default:
		return nil
	}
}
