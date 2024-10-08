package backend

import (
	"capi-bootstrap/providers/backend/github"
	"capi-bootstrap/providers/backend/s3"
)

func NewProvider(name string) Provider {
	switch name {
	case "s3":
		return s3.NewBackend()
	case "github":
		return github.NewBackend()
	default:
		return nil
	}
}

func ListProviders() []string {
	return []string{"s3", "github"}
}
