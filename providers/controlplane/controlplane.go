package controlplane

import "capi-bootstrap/providers/controlplane/k3s"

func NewControlPlaneProvider(name string) Provider {
	switch name {
	case "KThreesControlPlane":
		return k3s.K3s{}
	default:
		return nil
	}
}
