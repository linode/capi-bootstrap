package Linode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLinodeClient(t *testing.T) {
	token := "testToken"
	testClient := NewClient(token, context.Background())
	assert.NotNil(t, testClient)
}
