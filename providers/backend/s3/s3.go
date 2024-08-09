package s3

import (
	"context"
	"errors"

	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

func NewBackend() *Backend {
	return &Backend{
		Name: "s3",
	}
}

type Backend struct {
	Name string
}

func (b *Backend) PreCmd(_ context.Context, clusterName string) error {
	return errors.New("s3 not implemented")
}

func (b *Backend) Read(_ context.Context, clusterName string) (*v1.Config, error) {
	return nil, errors.New("s3 not implemented")
}

func (b *Backend) Write(_ context.Context, clusterName string, config *v1.Config) error {
	return errors.New("s3 not implemented")
}

func (b *Backend) Delete(_ context.Context, clusterName string) error {
	return errors.New("s3 not implemented")
}
