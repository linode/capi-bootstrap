package file

import (
	"context"
	"errors"

	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

const (
	Path = "capi-bootstrap"
)

type Backend struct{}

func (Backend) PreCmd(_ context.Context) error {
	return nil
}

func (Backend) Read(_ context.Context, providerName, clusterName string) (v1.Config, error) {
	return v1.Config{}, errors.New("file not implemented")
}

func (Backend) Write(_ context.Context, providerName, clusterName string, config v1.Config) error {
	return errors.New("file not implemented")
}
