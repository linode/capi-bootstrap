package infrastructure

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"capi-bootstrap/providers/infrastructure/linode"
)

func TestNewProvider(t *testing.T) {
	type test struct {
		name  string
		input string
		want  Provider
	}
	tests := []test{
		{name: "linode", input: "LinodeCluster", want: linode.NewInfrastructure()},
		{name: "not matching name", input: "wrong", want: nil},
		{name: "no name", input: "", want: nil},
	}
	for _, tc := range tests {
		actual := NewProvider(tc.input)
		assert.Equal(t, tc.want, actual)
	}
}
