package backend

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"capi-bootstrap/providers/backend/github"
	"capi-bootstrap/providers/backend/s3"
)

func TestNewProvider(t *testing.T) {
	type test struct {
		name  string
		input string
		want  Provider
	}
	tests := []test{
		{name: "file", input: "file", want: nil},
		{name: "s3", input: "s3", want: s3.NewBackend()},
		{name: "github", input: "github", want: github.NewBackend()},
		{name: "not matching name", input: "wrong", want: nil},
		{name: "no name", input: "", want: nil},
	}
	for _, tc := range tests {
		actual := NewProvider(tc.input)
		assert.Equal(t, tc.want, actual)
	}
}

func TestListProviders(t *testing.T) {
	backends := ListProviders()
	assert.Contains(t, backends, "s3")
	assert.Contains(t, backends, "github")
}
