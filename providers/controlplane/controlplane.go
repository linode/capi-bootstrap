package controlplane

import "capi-bootstrap/providers/controlplane/k3s"

func NewProvider(name string) Provider {
	switch name {
	case "KThreesControlPlane":
		return k3s.NewControlPlane()
	default:
		return nil
	}
}
