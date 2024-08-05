package infrastructure

import (
	Linode "capi-bootstrap/providers/infrastructure/linode"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProvider(t *testing.T) {
	type test struct {
		name  string
		input string
		want  Provider
	}
	tests := []test{
		{name: "linode", input: "LinodeCluster", want: Linode.CAPL{}},
		{name: "not matching name", input: "wrong", want: nil},
		{name: "no name", input: "", want: nil},
	}
	for _, tc := range tests {
		actual := NewProvider(tc.input)
		assert.Equal(t, tc.want, actual)
	}
}
