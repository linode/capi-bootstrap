package backend

import (
	"capi-bootstrap/providers/backend/file"
	"capi-bootstrap/providers/backend/s3"
)

func NewProvider(name string) Provider {
	switch name {
	case "s3":
		return s3.NewBackend()
	default:
		return file.NewBackend()
	}
}
