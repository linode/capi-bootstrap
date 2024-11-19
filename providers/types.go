package providers

import (
	"capi-bootstrap/providers/backend"
	"capi-bootstrap/providers/controlplane"
	"capi-bootstrap/providers/infrastructure"
)

type Providers struct {
	Infrastructure infrastructure.Provider
	ControlPlane   controlplane.Provider
	Backend        backend.Provider
}
