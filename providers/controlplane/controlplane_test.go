package controlplane

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"capi-bootstrap/providers/controlplane/k3s"
)

func TestNewProvider(t *testing.T) {
	type test struct {
		name  string
		input string
		want  Provider
	}
	tests := []test{
		{name: "k3s", input: "KThreesControlPlane", want: k3s.NewControlPlane()},
		{name: "not matching name", input: "wrong", want: nil},
		{name: "no name", input: "", want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actual := NewProvider(tc.input)
			assert.Equal(t, tc.want, actual)
		})
	}
}
