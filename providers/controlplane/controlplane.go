package controlplane

import (
	"capi-bootstrap/providers/controlplane/k3s"
	"capi-bootstrap/providers/controlplane/kubeadm"
)

func NewProvider(name string) Provider {
	switch name {
	case "KThreesControlPlane":
		return k3s.NewControlPlane()
	case "KubeadmControlPlane":
		return kubeadm.NewControlPlane()

	default:
		return nil
	}
}
