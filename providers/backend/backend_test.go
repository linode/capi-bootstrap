package backend

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"capi-bootstrap/providers/backend/file"
	"capi-bootstrap/providers/backend/s3"
)

func TestNewProvider(t *testing.T) {
	type test struct {
		name  string
		input string
		want  Provider
	}
	tests := []test{
		{name: "file", input: "file", want: file.NewBackend()},
		{name: "s3", input: "s3", want: s3.NewBackend()},
		{name: "not matching name", input: "wrong", want: file.NewBackend()},
		{name: "no name", input: "", want: file.NewBackend()},
	}
	for _, tc := range tests {
		actual := NewProvider(tc.input)
		assert.Equal(t, tc.want, actual)
	}
}
