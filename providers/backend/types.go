package backend

import (
	"context"

	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

type Provider interface {
	PreCmd(context.Context) error
	Read(context.Context, string, string) (v1.Config, error)
	Write(context.Context, string, string, v1.Config) error
}
